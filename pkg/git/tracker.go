package git

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
)

var (
	ErrGitNoRepos       = errors.New("git: no repos found")
	ErrGitNotTracked    = errors.New("git: repo not tracked")
	ErrGitRepoNameEmpty = errors.New("git: repo name is empty")
	ErrGitTracked       = errors.New("git: repo already tracked")
)

const TrackerFile = ".tracked.json"

// Tracker manages a list of tracked repositories stored in a file.
type Tracker struct {
	repos    []string // Repos holds tracked repository names or paths.
	filename string   // Filename is the path to the JSON file.
}

// NewTracker returns a new Tracker for the given root directory.
func NewTracker(destDir string) *Tracker {
	return &Tracker{
		filename: filepath.Join(destDir, TrackerFile),
	}
}

// Load loads the tracked repositories from the file (if exists).
func (t *Tracker) Load() error {
	if fileExists(t.filename) {
		return readFile(t.filename, &t.repos)
	}

	return nil
}

// Write writes the tracked repositories to the file.
func (t *Tracker) Write() error {
	t.repos = slices.Compact(t.repos)
	slog.Debug("writing tracker file", "repos", t.repos)
	return writeFile(t.filename, &t.repos)
}

// Track adds a new repository to the tracker.
func (t *Tracker) Track(names ...string) error {
	slog.Debug("adding tracker", "repos", names)
	if len(names) == 0 {
		return ErrGitRepoNameEmpty
	}
	t.repos = append(t.repos, names...)
	return nil
}

// Untrack removes a repository from the tracker.
func (t *Tracker) Untrack(name string) error {
	slog.Debug("untracking repo", "name", name)
	if name == "" {
		return ErrGitRepoNameEmpty
	}

	if !slices.Contains(t.repos, name) {
		slog.Debug("untrack repo not found", "name", name)
		return nil
	}

	t.repos = slices.DeleteFunc(
		t.repos,
		func(r string) bool {
			return r == name
		},
	)

	slog.Debug("result", "repos", t.repos)
	return nil
}

// Contains checks if a repository is tracked.
func (t *Tracker) Contains(name string) bool {
	return slices.Contains(t.repos, name)
}

// Repos returns the tracked repositories.
func (t *Tracker) Repos() []string {
	return t.repos
}
