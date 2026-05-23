package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrGitTracked        = errors.New("git: repo already tracked")
	ErrGitNotTracked     = errors.New("git: repo not tracked")
	ErrGitNoRepos        = errors.New("git: no repos found")
	ErrGitTrackNotLoaded = errors.New("git: tracker not loaded")
	ErrGitRepoNameEmpty  = errors.New("git: repo name is empty")
	ErrGitCurrentRepo    = errors.New("git: current repo not set")
)

const trackerFile = ".tracked.json"

// Tracker manages a list of tracked repositories stored in a file.
type Tracker struct {
	repos    []string // Repos holds tracked repository names or paths.
	filename string   // Filename is the path to the JSON file.
}

// NewTracker returns a new Tracker for the given root directory.
func NewTracker(root string) *Tracker {
	return &Tracker{
		filename: filepath.Join(root, trackerFile),
	}
}

// Load loads the tracked repositories from the file (if exists).
func (t *Tracker) Load() error {
	if files.Exists(t.filename) {
		if err := files.JSONRead(t.filename, &t.repos); err != nil {
			return fmt.Errorf("load tracker: %w", err)
		}
	}

	return nil
}

// Save writes the tracked repositories to the file.
func (t *Tracker) Save() error {
	t.repos = slices.Compact(t.repos)
	if _, err := files.JSONWrite(t.filename, &t.repos, true); err != nil {
		return fmt.Errorf("save tracker: %w", err)
	}

	return nil
}

// Sync loads and then saves, ensuring file consistency.
func (t *Tracker) Sync() error {
	if err := t.Load(); err != nil {
		return err
	}
	return t.Save()
}

// Track adds a new repository to the tracker.
func (t *Tracker) Track(name string) error {
	if name == "" {
		return ErrGitRepoNameEmpty
	}
	if slices.Contains(t.repos, name) {
		return ErrGitTracked
	}
	t.repos = append(t.repos, name)
	return nil
}

// Untrack removes a repository from the tracker.
func (t *Tracker) Untrack(name string) error {
	if name == "" {
		return ErrGitRepoNameEmpty
	}

	if !slices.Contains(t.repos, name) {
		return ErrGitNotTracked
	}

	t.repos = slices.DeleteFunc(
		t.repos,
		func(r string) bool {
			return r == name
		},
	)

	return nil
}

// Contains checks if a repository is tracked.
func (t *Tracker) Contains(name string) (bool, error) {
	return slices.Contains(t.repos, name), nil
}

// Repos returns the tracked repositories.
func (t *Tracker) Repos() []string {
	return t.repos
}
