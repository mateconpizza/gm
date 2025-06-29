// Package gitrepo provides the model and logic of a bookmarks Git repository.
package git

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var (
	ErrNoManager  = errors.New("manager is required")
	ErrNoRepoPath = errors.New("repoPath is required")
	ErrNoDBPath   = errors.New("database path is required")
)

// Location holds all path and naming information for a repository.
type Location struct {
	Name   string // Database name without extension (e.g., "bookmarks")
	DBName string // Database base name (e.g., "bookmarks.db")
	DBPath string // Database fullpath (e.g., "/home/user/.local/share/app/bookmarks.db")
	Git    string // Path to where to store the Git repository (e.g., "/home/user/.local/share/app/git")
	Path   string // Path to where to store the associated Git files (e.g., "/home/user/.local/share/app/git/bookmarks")
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
		Hash:   txt.GenHashPath(dbPath),
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
	if gr.IsEncrypted() {
		if _, err := exportAsGPG(gr.Loc.Path, bs); err != nil {
			return err
		}
	}

	if _, err := exportAsJSON(gr.Loc.Path, bs); err != nil {
		return err
	}

	return nil
}

// Update updates the bookmarks in the repo.
func (gr *Repository) Update(oldB, newB *bookmark.Bookmark) error {
	if err := gr.Remove([]*bookmark.Bookmark{oldB}); err != nil {
		return err
	}

	return gr.Add([]*bookmark.Bookmark{newB})
}

// Remove removes the bookmarks from the repo.
func (gr *Repository) Remove(bs []*bookmark.Bookmark) error {
	if gr.IsEncrypted() {
		return cleanGPGRepo(gr.Loc.Path, bs)
	}

	return cleanJSONRepo(gr.Loc.Path, bs)
}

// Drop drops the repo.
func (gr *Repository) Drop(mesg string) error {
	if err := files.RemoveAll(gr.Loc.Path); err != nil {
		return err
	}
	if err := gr.Git.AddAll(); err != nil {
		return err
	}

	return gr.Git.Commit(mesg)
}

func (gr *Repository) Commit(msg string) error {
	return commitIfChanged(gr, msg)
}

// Stats returns the repo stats.
func (gr *Repository) Stats() (*SyncGitSummary, error) {
	sum := NewSummary()
	if err := repoStats(gr.Loc.DBPath, sum); err != nil {
		return nil, err
	}

	return sum, nil
}

// Summary returns the repo summary.
func (gr *Repository) Summary() (*SyncGitSummary, error) {
	return syncSummary(gr)
}

// SummaryWrite writes the repo stats.
func (gr *Repository) SummaryWrite() error {
	return writeRepoStats(gr)
}

// Write writes the files to the git repo.
func (gr *Repository) Write(bs []*bookmark.Bookmark) (bool, error) {
	if gr.IsEncrypted() {
		return exportAsGPG(gr.Loc.Path, bs)
	}

	return exportAsJSON(gr.Loc.Path, bs)
}

func (gr *Repository) Read(c *ui.Console) ([]*bookmark.Bookmark, error) {
	if gr.IsEncrypted() {
		return readGPGRepo(c, gr.Loc.Path)
	}

	return readJSONRepo(c, gr.Loc.Path)
}

func (gr *Repository) Track() error {
	return gr.Tracker.Track(gr.Loc.Hash).Save()
}

func (gr *Repository) Untrack() error {
	return gr.Tracker.Untrack(gr.Loc.Hash).Save()
}

// IsEncrypted returns whether the repo is encrypted.
func (gr *Repository) IsEncrypted() bool {
	return gpg.IsInitialized(gr.Git.RepoPath)
}

// IsTracked returns whether the repo is tracked.
func (gr *Repository) IsTracked() bool {
	return gr.Tracker.Contains(gr.Loc.Hash)
}

// Export exports the repo.
func (gr *Repository) Export() error {
	bs, err := records(gr.Loc.DBPath)
	if err != nil {
		return err
	}

	if _, err := gr.Write(bs); err != nil {
		return err
	}

	return nil
}

// Records gets all records from the database associated with the repo.
func (gr *Repository) Records() ([]*bookmark.Bookmark, error) {
	return records(gr.Loc.DBPath)
}

// TrackAndCommit tracks and commits the repo.
func (gr *Repository) TrackAndCommit() error {
	if err := gr.Export(); err != nil {
		return err
	}
	if err := gr.Track(); err != nil {
		return err
	}

	return gr.Commit("new tracking")
}

// String returns the repo summary.
func (gr *Repository) String() string {
	sum, err := gr.Stats()
	if err != nil {
		slog.Error("error getting repo summary", "error", err)
		return ""
	}

	return repoSummaryString(sum)
}

// Read reads the repo and returns the bookmarks.
func Read(c *ui.Console, path string) ([]*bookmark.Bookmark, error) {
	if gpg.IsInitialized(path) {
		return readGPGRepo(c, path)
	}

	return readJSONRepo(c, path)
}
