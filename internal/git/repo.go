// Package git provides the model and logic of a bookmarks Git repository.
package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrNoManager  = errors.New("manager is required")
	ErrNoRepoPath = errors.New("repoPath is required")
	ErrNoDBPath   = errors.New("database path is required")
)

// Location holds all path and naming information for a repository.
type Location struct {
	Name   string // Database name without extension (e.g., "bookmarks")
	DBName string // Database base name (e.g., "main.db")
	DBPath string // Database fullpath (e.g., "/home/user/.local/share/app/main.db")
	Git    string // Path to where to store the Git repository (e.g., "~/.local/share/gomarks/git")
	Path   string // Path to where to store the associated Git files (e.g., "~/.local/share/gomarks/git/bookmarks")
	Hash   string // Hash of the database fullpath (for internal lookups/storage)
}

// Repository represents a bookmarks repository.
type Repository struct {
	Loc     *Location // Encapsulates all path and naming details
	Tracker *Tracker  // Git repo tracker
	Git     *Manager  // Git manager
}

// newLocation creates a new Location.
func newLocation(dbPath string) *Location {
	baseName := filepath.Base(dbPath)
	name := files.StripSuffixes(baseName)
	git := filepath.Join(filepath.Dir(dbPath), "git")

	return &Location{
		Name:   name,
		DBName: baseName,
		DBPath: dbPath,
		Git:    git,
		Path:   filepath.Join(git, name),
		Hash:   name,
	}
}

func NewRepo(dbPath string) (*Repository, error) {
	if dbPath == "" {
		return nil, ErrNoDBPath
	}

	loc := newLocation(dbPath)
	tk := NewTracker(loc.Git)
	if err := tk.Load(); err != nil {
		return nil, err
	}

	gitCmd, err := sys.Which("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, "git")
	}

	return &Repository{
		Loc:     loc,
		Tracker: tk,
		Git:     NewGit(loc.Git, WithCmd(gitCmd)),
	}, nil
}

// Add adds the bookmarks to the repo.
func (gr *Repository) Add(bs []*bookmark.Bookmark) error {
	if _, err := gr.Write(bs); err != nil {
		return err
	}

	return nil
}

// UpdateOne updates the bookmarks in the repo.
func (gr *Repository) UpdateOne(oldB, newB *bookmark.Bookmark) error {
	if err := gr.Remove([]*bookmark.Bookmark{oldB}); err != nil {
		return err
	}

	return gr.Add([]*bookmark.Bookmark{newB})
}

// Remove removes the bookmarks from the repo.
func (gr *Repository) Remove(bs []*bookmark.Bookmark) error {
	if gr.IsEncrypted() {
		return cleanGPGRepo(gr.Git.ctx, gr.Loc.Path, bs)
	}

	return cleanJSONRepo(gr.Git.ctx, gr.Loc.Path, bs)
}

// Drop removes a repository's files, updates its summary, and commits the
// changes.
func (gr *Repository) Drop(mesg string) error {
	return dropRepo(gr, mesg)
}

// Commit commits the bookmarks to the git repo.
func (gr *Repository) Commit(msg string) error {
	return commitIfChanged(gr.Git.ctx, gr, msg)
}

// Stats returns the repo stats.
func (gr *Repository) Stats() (*SyncGitSummary, error) {
	sum := NewSummary()
	if err := repoStats(gr.Git.ctx, gr.Loc.DBPath, sum); err != nil {
		return nil, err
	}

	return sum, nil
}

// Summary returns the repo summary.
func (gr *Repository) Summary() (*SyncGitSummary, error) {
	return summaryRead(gr)
}

// SummaryUpdate returns a new SyncGitSummary.
func (gr *Repository) SummaryUpdate(version string) (*SyncGitSummary, error) {
	return summaryUpdate(gr.Git.ctx, gr, version)
}

// RepoStatsWrite calculates, updates, and saves the repository's statistics to
// its summary file.
func (gr *Repository) RepoStatsWrite() error {
	return writeRepoStats(gr.Git.ctx, gr)
}

// Write exports the provided bookmarks to the repository's file, encrypting if
// configured.
func (gr *Repository) Write(bs []*bookmark.Bookmark) (bool, error) {
	if gr.IsEncrypted() {
		fingerprintPath := gpg.GPGIDPath(gr.Loc.Git)
		return exportAsGPG(gr.Git.ctx, fingerprintPath, gr.Loc.Path, bs)
	}

	return exportAsJSON(gr.Loc.Path, bs)
}

// Read reads and decrypts the repository's bookmarks, handling encryption if
// configured.
// func (gr *Repository) Read(c *ui.Console) ([]*bookmark.Bookmark, error) {
// 	return Read(c, gr.Loc.Path)
// }

// Track tracks a repository in Git, exporting its data and committing the
// changes.
func (gr *Repository) Track() error {
	return trackRepo(gr)
}

// Untrack untracks a repository in Git, removes its files, and
// commits the change.
func (gr *Repository) Untrack(mesg string) error {
	return untrackRemoveRepo(gr, mesg)
}

// IsEncrypted returns whether the repo is encrypted.
func (gr *Repository) IsEncrypted() bool {
	return gpg.IsInitialized(gr.Git.RepoPath)
}

// IsTracked returns whether the repo is tracked.
func (gr *Repository) IsTracked() bool {
	ok, _ := gr.Tracker.Contains(gr.Loc.Hash)
	return ok
}

// Export exports the repository's bookmarks to Git, handling encryption if
// configured.
func (gr *Repository) Export() error {
	bs, err := records(gr.Git.ctx, gr.Loc.DBPath)
	if err != nil {
		return err
	}

	if _, err := gr.Write(bs); err != nil {
		return err
	}

	return nil
}

// Records retrieves all bookmarks from the repository's database.
func (gr *Repository) Records() ([]*bookmark.Bookmark, error) {
	return records(gr.Git.ctx, gr.Loc.DBPath)
}

// String returns the repo summary.
func (gr *Repository) String() string {
	sum, err := gr.Stats()
	if err != nil {
		slog.Error("error getting repo summary", "error", err)
		return ""
	}

	return sum.RepoStats.String()
}

// AskForEncryption prompts the user to enable GPG encryption for the
// repository if it's not already encrypted.
func (gr *Repository) AskForEncryption(c *ui.Console) error {
	if gr.IsEncrypted() {
		return nil
	}

	_, err := sys.Which(gpg.Command)
	if err != nil {
		slog.Debug("git repo with GPG, command not found", "command", gpg.Command)
		//nolint:nilerr //test
		return nil
	}

	c.Frame().Success("GPG command found").Ln().Flush()
	if !c.Confirm("Use GPG for encryption?", "n") {
		return nil
	}

	fps, err := gpg.ListFingerprints()
	if err != nil {
		return err
	}

	key, err := selectFingerprint(c, fps)
	if err != nil {
		return err
	}

	return initGPG(c, gr, key)
}

// Status returns a prettify status of the repository.
func (gr *Repository) Status(c *ui.Console) string {
	return repoStatus(c, gr)
}

// SetConfig sets the app git config.
func SetConfig(ctx context.Context, c *config.Config) {
	c.Git.Path = filepath.Join(c.Path.Data, "git")
	c.Git.Enabled = IsInitialized(c.Git.Path)
	c.Git.GPG = gpg.IsInitialized(c.Git.Path)
	remote, _ := Remote(ctx, c.Git.Path)
	c.Git.Remote = remote
}

func selectFingerprint(c *ui.Console, fps []*gpg.Fingerprint) (*gpg.Fingerprint, error) {
	p := c.Palette()
	trustColor := func(key *gpg.Fingerprint) string {
		t := key.TrustLevelString()
		if key.IsTrusted() {
			return p.BrightGreen(strings.ToUpper(t))
		}

		switch t {
		case "marginal":
			return p.BrightOrange(strings.ToUpper(t))
		default:
			return p.BrightRed(strings.ToUpper(t))
		}
	}

	m := menu.New[*gpg.Fingerprint](
		menu.WithArgs("--no-bold"),
		menu.WithColor(p.Enabled()),
		menu.WithHeader("select a fingerprint"),
		menu.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
		menu.WithMultilineView(),
		menu.WithPreview(gpg.Command+" --list-keys {+4}"),
	)
	m.SetItems(fps)
	m.SetPreprocessor(func(f **gpg.Fingerprint) string {
		fp := *f
		return fmt.Sprintf(
			"[Trusted: %s] %s: %s %s: %s\n%s: %s",
			trustColor(fp),
			p.BrightBlueBold("KeyID"),
			fp.KeyID,
			p.BrightMagentaBold("UserID"),
			fp.UserID,
			p.BrightYellowBold("Fingerprint"),
			fp.Fingerprint,
		)
	})

	keys, err := m.Select()
	if err != nil {
		return nil, err
	}

	key := keys[0]

	return key, nil
}
