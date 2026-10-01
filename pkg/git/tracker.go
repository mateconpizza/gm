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
	file *JSONFile[[]string] // ["main", "work", "org", ... ]
}

// newTracker returns a new Tracker for the given root directory.
func newTracker(destDir string) *Tracker {
	return &Tracker{
		file: newJSONFile[[]string](filepath.Join(destDir, TrackerFile)),
	}
}

// load loads the tracked repositories from the file (if exists).
func (t *Tracker) load() error {
	if t.file.exists() {
		return t.file.read()
	}
	return nil
}

func (t *Tracker) contains(name string) bool { return slices.Contains(t.file.value, name) }
func (t *Tracker) list() []string            { return t.file.value }
func (t *Tracker) reset()                    { t.file.value = make([]string, 0) }

// write writes the tracked repositories to the file.
func (t *Tracker) write() error {
	slices.Sort(t.file.value)
	t.file.value = slices.Compact(t.file.value)
	slog.Debug("writing tracker file", "repos", t.file.value)
	return t.file.write()
}

// track adds a new repository to the tracker.
func (t *Tracker) track(names ...string) error {
	slog.Debug("adding tracker", "repos", names)
	if len(names) == 0 {
		return ErrGitRepoNameEmpty
	}
	t.file.value = append(t.file.value, names...)
	return nil
}

// untrack removes a repository from the tracker.
func (t *Tracker) untrack(name string) error {
	slog.Debug("untracking repo", "name", name)
	if name == "" {
		return ErrGitRepoNameEmpty
	}

	if !slices.Contains(t.file.value, name) {
		slog.Debug("untrack repo not found", "name", name)
		return nil
	}

	t.file.value = slices.DeleteFunc(
		t.file.value,
		func(r string) bool {
			return r == name
		},
	)

	slog.Debug("result", "repos", t.file.value)
	return nil
}
