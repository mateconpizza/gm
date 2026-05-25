// Package git provides the model and logic of a bookmarks Git repository.
package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrNoDBPath = errors.New("database path is required")

// RepoManager represents a bookmarks repository.
type RepoManager struct {
	Loc     *Location // Encapsulates all path and naming details
	Tracker *Tracker  // Git repo tracker
	Git     *Git      // Git manager
}

// Location holds all path and naming information for a repository.
type Location struct {
	Name   string // Database name without extension (e.g., "main")
	DBName string // Database base name (e.g., "main.db")
	DBPath string // Database fullpath (e.g., "/home/user/.local/share/app/main.db")
	Git    string // Path to where to store the Git repository (e.g., "~/.local/share/gomarks/git")
	Path   string // Path to where to store the associated Git files (e.g., "~/.local/share/gomarks/git/main")
	Hash   string // Hash of the database fullpath (for internal lookups/storage)
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

func NewManager(dbPath string) (*RepoManager, error) {
	if dbPath == "" {
		return nil, ErrNoDBPath
	}

	loc := newLocation(dbPath)
	t := NewTracker(loc.Git)
	if err := t.Load(); err != nil {
		return nil, err
	}

	gitCmd, err := which("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, "git")
	}

	return &RepoManager{
		Loc:     loc,
		Tracker: t,
		Git:     newGit(loc.Git, WithCmd(gitCmd)),
	}, nil
}

// Add adds the bookmarks to the repo.
func (m *RepoManager) Add(bs []*bookmark.Bookmark) error {
	if _, err := m.Write(bs, false); err != nil {
		return err
	}

	return nil
}

// UpdateOne updates the bookmarks in the repo.
func (m *RepoManager) UpdateOne(oldB, newB *bookmark.Bookmark) error {
	if err := m.Remove([]*bookmark.Bookmark{oldB}); err != nil {
		return err
	}

	return m.Add([]*bookmark.Bookmark{newB})
}

// Remove removes the bookmarks from the repo.
func (m *RepoManager) Remove(bs []*bookmark.Bookmark) error {
	if m.IsEncrypted() {
		return cleanGPGRepo(m.Git.ctx, m.Loc.Path, bs)
	}

	return cleanJSONRepo(m.Git.ctx, m.Loc.Path, bs)
}

// Drop removes a repository's files, updates its summary, and commits the
// changes.
func (m *RepoManager) Drop(mesg string) error {
	return dropRepo(m, mesg)
}

// Commit commits the bookmarks to the git repo.
func (m *RepoManager) Commit(mesg string) error {
	slog.DebugContext(m.Git.ctx, "git: committing changes to git", "message", mesg)
	return commitIfChanged(m.Git.ctx, m, mesg)
}

// Stats returns the repo stats.
func (m *RepoManager) Stats() (*SyncGitSummary, error) {
	sum := NewSummary()
	if err := repoStats(m.Git.ctx, m.Loc.DBPath, sum); err != nil {
		return nil, err
	}

	return sum, nil
}

// Summary returns the repo summary.
func (m *RepoManager) Summary() (*SyncGitSummary, error) {
	return summaryRead(m)
}

// SummaryUpdate returns a new SyncGitSummary.
func (m *RepoManager) SummaryUpdate(version string) (*SyncGitSummary, error) {
	return summaryUpdate(m.Git.ctx, m, version)
}

// WriteStats calculates, updates, and saves the repository's statistics to
// its summary file.
func (m *RepoManager) WriteStats() error {
	return writeRepoStats(m.Git.ctx, m)
}

// Write exports the provided bookmarks to the repository's file, encrypting if
// configured.
func (m *RepoManager) Write(bs []*bookmark.Bookmark, force bool) (bool, error) {
	slog.Debug("git: writing bookmarks to git repo", "force", force)
	if m.IsEncrypted() {
		fingerprintPath := gpg.GPGIDPath(m.Loc.Git)
		return exportAsGPG(m.Git.ctx, fingerprintPath, m.Loc.Path, bs)
	}

	return exportAsJSON(m.Loc.Path, bs, force)
}

// Read reads and decrypts the repository's bookmarks, handling encryption if
// configured.
// func (m *RepoManager) Read(c *ui.Console) ([]*bookmark.Bookmark, error) {
// 	return Read(c, m.Loc.Path)
// }

// Track tracks a repository in Git, exporting its data and committing the
// changes.
func (m *RepoManager) Track() error {
	return trackRepo(m)
}

// Untrack untracks a repository in Git, removes its files, and
// commits the change.
func (m *RepoManager) Untrack(mesg string) error {
	return untrackRemoveRepo(m, mesg)
}

// IsEncrypted returns whether the repo is encrypted.
func (m *RepoManager) IsEncrypted() bool {
	return gpg.IsInitialized(m.Git.repoPath)
}

// IsTracked returns whether the repo is tracked.
func (m *RepoManager) IsTracked() bool {
	ok, _ := m.Tracker.Contains(m.Loc.Hash)
	return ok
}

// Export exports the repository's bookmarks to Git, handling encryption if
// configured.
func (m *RepoManager) Export() error {
	bs, err := records(m.Git.ctx, m.Loc.DBPath)
	if err != nil {
		return err
	}

	if _, err := m.Write(bs, false); err != nil {
		return err
	}

	return nil
}

// Records retrieves all bookmarks from the repository's database.
func (m *RepoManager) Records() ([]*bookmark.Bookmark, error) {
	return records(m.Git.ctx, m.Loc.DBPath)
}

// String returns the repo summary.
func (m *RepoManager) String() string {
	sum, err := m.Stats()
	if err != nil {
		slog.Error("error getting repo summary", "error", err)
		return ""
	}

	return sum.RepoStats.String()
}

// AskForEncryption prompts the user to enable GPG encryption for the
// repository if it's not already encrypted.
func (m *RepoManager) AskForEncryption(c *ui.Console, app *application.App) error {
	if m.IsEncrypted() {
		return nil
	}

	_, err := which(gpg.Command)
	if err != nil {
		slog.Debug("git repo with GPG, command not found", "command", gpg.Command)
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
	}

	c.Frame().Success("GPG command found").Ln().Flush()
	if !c.Confirm("Use GPG for encryption? "+c.Palette().BrightRed.Wrap("(experimental)", c.Palette().Italic), "n") {
		return nil
	}

	fps, err := gpg.ListFingerprints()
	if err != nil {
		return err
	}

	mf := menuFingerprint(c, app)
	key, err := selectFingerprint(mf, fps)
	if err != nil {
		return err
	}

	return initGPG(c, m, key)
}

// Status returns a prettify status of the repository.
func (m *RepoManager) Status(c *ui.Console) string {
	return repoStatus(c, m)
}

// SetConfig sets the app git config.
func SetConfig(ctx context.Context, c *application.App) {
	c.Git.Path = filepath.Join(c.Path.Data, "git")
	c.Git.Enabled = IsInitialized(c.Git.Path)
	c.Git.GPG = gpg.IsInitialized(c.Git.Path)
	remote, _ := Remote(ctx, c.Git.Path)
	c.Git.Remote = remote
}

func selectFingerprint(m *menu.Menu[*gpg.Fingerprint], fps []*gpg.Fingerprint) (*gpg.Fingerprint, error) {
	keys, err := m.Select(fps)
	if err != nil {
		return nil, err
	}

	key := keys[0]

	return key, nil
}

func menuFingerprint(c *ui.Console, app *application.App) *menu.Menu[*gpg.Fingerprint] {
	p := c.Palette()
	trustColor := func(key *gpg.Fingerprint) string {
		t := key.TrustLevelString()
		if key.IsTrusted() {
			return p.BrightGreen.Sprint(strings.ToUpper(t))
		}

		switch t {
		case "marginal":
			return p.BrightYellow.Sprint(strings.ToUpper(t))
		default:
			return p.BrightRed.Sprint(strings.ToUpper(t))
		}
	}

	m := picker.New[*gpg.Fingerprint](
		app,
		menu.WithArgs("--no-bold"),
		menu.WithHeader(" select a fingerprint "),
		menu.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
		menu.WithMultilineView(),
		menu.WithPreview(gpg.Command+" --list-keys {+4}"),
	)
	m.SetFormatter(func(f **gpg.Fingerprint) string {
		fp := *f
		return fmt.Sprintf(
			"[Trusted: %s] %s: %s %s: %s\n%s: %s",
			trustColor(fp),
			p.BrightBlue.Wrap("KeyID", p.Bold),
			fp.KeyID,
			p.BrightMagenta.Wrap("UserID", p.Bold),
			fp.UserID,
			p.BrightYellow.Wrap("Fingerprint", p.Bold),
			fp.Fingerprint,
		)
	})

	return m
}
