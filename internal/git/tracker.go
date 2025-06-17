package git

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var (
	ErrGitTracked        = errors.New("git: repo already tracked")
	ErrGitNotTracked     = errors.New("git: repo not tracked")
	ErrGitNoTrackedRepos = errors.New("git: no tracked repos found")
	ErrGitTrackNotLoaded = errors.New("git: tracker not loaded")
)

const filepathTracked = ".tracked.json"

type Tracker struct {
	List     []string
	loaded   bool
	Filename string
	current  *GitRepository
}

// GitRepository represents a bookmarks repository.
type GitRepository struct {
	DBName   string // Database base name
	DBPath   string // Database fullpath
	Name     string // Database name without ext
	Path     string // Path to where to store the files
	HashPath string // Database fullpath hash
}

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

func (t *Tracker) Track(gr *GitRepository) *Tracker {
	t.List = append(t.List, gr.HashPath)

	return t
}

func (t *Tracker) Untrack(gr *GitRepository) *Tracker {
	t.List = slices.DeleteFunc(t.List, func(r string) bool {
		return r == gr.HashPath
	})

	return t
}

func (t *Tracker) Save() error {
	if !t.loaded {
		return ErrGitTrackNotLoaded
	}

	if err := files.JSONWrite(t.Filename, &t.List, true); err != nil {
		return fmt.Errorf("%w: %q", err, t.Filename)
	}

	return nil
}

func (t *Tracker) IsTracked(gr *GitRepository) bool {
	if !t.loaded {
		slog.Error("tracker not loaded")
	}
	return slices.Contains(t.List, gr.HashPath)
}

func (t *Tracker) Current() *GitRepository {
	if t.current == nil {
		panic("current repo not set")
	}

	return t.current
}

func (t *Tracker) SetCurrent(gr *GitRepository) {
	t.current = gr
}

func NewTracker(root string) *Tracker {
	return &Tracker{
		Filename: filepath.Join(root, filepathTracked),
	}
}

// Tracked returns the tracked repositories.
func Tracked(trackerFile string) ([]string, error) {
	tracked := make([]string, 0)

	if !files.Exists(trackerFile) {
		if err := files.JSONWrite(trackerFile, &tracked, true); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		return tracked, nil
	}

	if err := files.JSONRead(trackerFile, &tracked); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return tracked, nil
}

func IsTracked(repoPath, dbPath string) bool {
	t := filepath.Join(repoPath, filepathTracked)
	tracked, err := Tracked(t)
	if err != nil {
		return false
	}

	gr := newGitRepository(repoPath, dbPath)

	return slices.Contains(tracked, gr.HashPath)
}

func newGitRepository(repoPath, dbPath string) *GitRepository {
	baseName := filepath.Base(dbPath)
	name := files.StripSuffixes(baseName)

	return &GitRepository{
		DBName:   baseName,
		DBPath:   dbPath,
		Name:     name,
		Path:     filepath.Join(repoPath, name),
		HashPath: txt.GenHashPath(dbPath),
	}
}
