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
	Repos    []string // Repos holds tracked repository names or paths.
	Filename string   // Filename is the path to the JSON file.
	loaded   bool
}

// NewTracker returns a new Tracker for the given root directory.
func NewTracker(root string) *Tracker {
	return &Tracker{
		Filename: filepath.Join(root, trackerFile),
	}
}

// Load loads the tracked repositories from the file (if exists).
func (t *Tracker) Load() error {
	if t.loaded {
		return nil
	}

	if files.Exists(t.Filename) {
		if err := files.JSONRead(t.Filename, &t.Repos); err != nil {
			return fmt.Errorf("load tracker: %w", err)
		}
	}

	t.loaded = true
	return nil
}

// Save writes the tracked repositories to the file.
func (t *Tracker) Save() error {
	if !t.loaded {
		return ErrGitTrackNotLoaded
	}

	t.Repos = slices.Compact(t.Repos)
	if _, err := files.JSONWrite(t.Filename, &t.Repos, true); err != nil {
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
	if !t.loaded {
		return ErrGitTrackNotLoaded
	}
	if slices.Contains(t.Repos, name) {
		return ErrGitTracked
	}
	t.Repos = append(t.Repos, name)
	return nil
}

// Untrack removes a repository from the tracker.
func (t *Tracker) Untrack(name string) error {
	if name == "" {
		return ErrGitRepoNameEmpty
	}
	if !t.loaded {
		return ErrGitTrackNotLoaded
	}
	if !slices.Contains(t.Repos, name) {
		return ErrGitNotTracked
	}
	t.Repos = slices.DeleteFunc(t.Repos, func(r string) bool { return r == name })
	return nil
}

// Contains checks if a repository is tracked.
func (t *Tracker) Contains(name string) (bool, error) {
	if !t.loaded {
		return false, ErrGitTrackNotLoaded
	}

	return slices.Contains(t.Repos, name), nil
}

// Tracked returns the currently tracked repositories.
func Tracked(path string) ([]string, error) {
	tracked := []string{}
	if !files.Exists(path) {
		if _, err := files.JSONWrite(path, &tracked, true); err != nil {
			return nil, fmt.Errorf("create tracker: %w", err)
		}
		return tracked, nil
	}
	if err := files.JSONRead(path, &tracked); err != nil {
		return nil, fmt.Errorf("read tracker: %w", err)
	}
	return tracked, nil
}
