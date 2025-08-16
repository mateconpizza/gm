package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mateconpizza/gm/internal/sys/files"
)

var (
	ErrGitTracked        = errors.New("git: repo already tracked")
	ErrGitNotTracked     = errors.New("git: repo not tracked")
	ErrGitNoRepos        = errors.New("git: no repos found")
	ErrGitTrackNotLoaded = errors.New("git: tracker not loaded")
	ErrGitRepoNameEmpty  = errors.New("git: repo name is empty")
	ErrGitCurrentRepo    = errors.New("git: current repo not set")
)

const TrackerFilepath = ".tracked.json"

// Tracker manages a list of tracked repositories stored in a file.
type Tracker struct {
	List     []string // List holds the paths of the tracked repositories.
	loaded   bool     // loaded indicates if the tracker data has been loaded from the file.
	Filename string   // Filename is the path to the file where the tracker data is stored.
}

// Load loads the repositories from the tracker file.
func (t *Tracker) Load() error {
	if t.loaded {
		return nil
	}

	if files.Exists(t.Filename) {
		if err := files.JSONRead(t.Filename, &t.List); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	t.loaded = true

	return nil
}

// Track adds the repo to the tracker.
func (t *Tracker) Track(s string) *Tracker {
	t.List = append(t.List, s)
	return t
}

// Untrack removes the repo from the tracker.
func (t *Tracker) Untrack(s string) *Tracker {
	t.List = slices.DeleteFunc(t.List, func(r string) bool {
		return r == s
	})

	return t
}

// Save saves the tracker.
func (t *Tracker) Save() error {
	if !t.loaded {
		return ErrGitTrackNotLoaded
	}

	t.List = slices.Compact(t.List)

	if _, err := files.JSONWrite(t.Filename, &t.List, true); err != nil {
		return fmt.Errorf("%w: %q", err, t.Filename)
	}

	return nil
}

// Contains returns true if the repo is tracked.
func (t *Tracker) Contains(s string) bool {
	if !t.loaded {
		panic(ErrGitTrackNotLoaded)
	}

	return slices.Contains(t.List, s)
}

func NewTracker(root string) *Tracker {
	return &Tracker{
		Filename: filepath.Join(root, TrackerFilepath),
	}
}

// Tracked returns the tracked repositories.
func Tracked(trackerFile string) ([]string, error) {
	tracked := make([]string, 0)

	if !files.Exists(trackerFile) {
		if _, err := files.JSONWrite(trackerFile, &tracked, true); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		return tracked, nil
	}

	if err := files.JSONRead(trackerFile, &tracked); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return tracked, nil
}
