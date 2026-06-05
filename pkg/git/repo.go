package git

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrIgnoreFilepath  = errors.New("git: ignore filepath")
	ErrNoFunctionFound = errors.New("git: no function provided")
	ErrNoStoreFound    = errors.New("git: no store found")
)

type RepoDB interface {
	Stats(ctx context.Context, dest any) error
}

type (
	ReaderFunc      func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error)
	WriterFunc      func(ctx context.Context, path string, bs []*bookmark.Bookmark) error
	RemoverFunc     func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error
	PostRemovalFunc func(path string) error
)

type RepoOptFunc func(*RepoOptions)

type RepoOptions struct {
	reader  ReaderFunc
	writer  WriterFunc
	remover RemoverFunc
	db      RepoDB
}

func WithRepoWriter(w WriterFunc) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.writer = w
	}
}

func WithRepoReader(r ReaderFunc) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.reader = r
	}
}

func WithRepoRemover(rm RemoverFunc) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.remover = rm
	}
}

func WithRepoStore(store RepoDB) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.db = store
	}
}

type Repo struct {
	name        string
	fullpath    string
	summaryFile string
	bookmarks   []*bookmark.Bookmark

	*RepoOptions
}

func (gr *Repo) Name() string     { return gr.name }
func (gr *Repo) Root() string     { return filepath.Dir(gr.fullpath) }
func (gr *Repo) DB() RepoDB       { return gr.db }
func (gr *Repo) Fullpath() string { return gr.fullpath }

func (gr *Repo) String() string {
	stats, err := gr.Stats()
	if err != nil {
		slog.Error("error getting repo summary", "error", err)
		return ""
	}
	return stats.String()
}

func (gr *Repo) Bookmarks() []*bookmark.Bookmark {
	slices.SortFunc(gr.bookmarks, func(a, b *bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return gr.bookmarks
}

func (gr *Repo) Add(ctx context.Context, bs []*bookmark.Bookmark) error {
	if gr.writer == nil {
		return fmt.Errorf("%w: file writer", ErrNoFunctionFound)
	}

	if err := gr.writer(ctx, gr.fullpath, bs); err != nil {
		return err
	}

	gr.bookmarks = append(gr.bookmarks, bs...)

	return nil
}

func (gr *Repo) RmMany(ctx context.Context, bs []*bookmark.Bookmark, postRm PostRemovalFunc) error {
	if gr.remover == nil {
		return fmt.Errorf("%w: file remover", ErrNoFunctionFound)
	}

	if err := gr.remover(ctx, gr.fullpath, bs); err != nil {
		return err
	}

	// removal map
	toRemove := make(map[string]bool, len(bs))
	for _, b := range bs {
		toRemove[b.URL] = true
	}

	gr.bookmarks = slices.DeleteFunc(gr.bookmarks, func(e *bookmark.Bookmark) bool {
		return toRemove[e.URL]
	})

	if err := postRm(gr.Fullpath()); err != nil {
		return fmt.Errorf("post removal function: %q: %w", gr.Fullpath(), err)
	}

	return nil
}

func (gr *Repo) Rm(ctx context.Context, b *bookmark.Bookmark, postRemoval PostRemovalFunc) error {
	if gr.remover == nil {
		return fmt.Errorf("%w: file remover", ErrNoFunctionFound)
	}

	if err := gr.remover(ctx, gr.Fullpath(), []*bookmark.Bookmark{b}); err != nil {
		return fmt.Errorf("remove bookmark %q from repo %q: %w", b.ID, gr.Fullpath(), err)
	}

	gr.bookmarks = slices.DeleteFunc(
		gr.bookmarks,
		func(e *bookmark.Bookmark) bool {
			return e.ID == b.ID
		},
	)

	if err := postRemoval(gr.Fullpath()); err != nil {
		return fmt.Errorf("post removal function: %q: %w", gr.Fullpath(), err)
	}

	return nil
}

func (gr *Repo) Read(ctx context.Context) error {
	if gr.reader == nil {
		return fmt.Errorf("%w: file reader", ErrNoFunctionFound)
	}

	total, err := gr.Count()
	if err != nil {
		return err
	}

	bs, err := gr.reader(ctx, gr.fullpath, total)
	if err != nil {
		return err
	}

	gr.bookmarks = bs
	return nil
}

// Count returns bookmark's count.
func (gr *Repo) Count() (int, error) {
	sum, err := gr.Summary()
	if err != nil {
		return 0, err
	}

	if sum.RepoStats == nil {
		return 0, err
	}

	return sum.RepoStats.Bookmarks, nil
}

// Summary returns current summary from the git repository.
func (gr *Repo) Summary() (*Summary, error) {
	sum := NewSummary()

	if !fileExists(gr.summaryFile) {
		return sum, nil
	}

	if err := readFile(gr.summaryFile, sum); err != nil {
		return nil, err
	}

	return sum, nil
}

// Stats returns current stats from the git repository.
func (gr *Repo) Stats() (*RepoStats, error) {
	if !fileExists(gr.summaryFile) {
		return &RepoStats{}, nil
	}

	sum := NewSummary()
	err := readFile(gr.summaryFile, &sum)
	sum.RepoStats.Name = gr.Name()
	return sum.RepoStats, err
}

// StatsFromDB returns fresh stats from the current database.
func (gr *Repo) StatsFromDB(ctx context.Context, db RepoDB) (*RepoStats, error) {
	stats := NewRepoStats()
	if err := db.Stats(ctx, stats); err != nil {
		return nil, err
	}
	stats.Name = gr.Name()

	return stats, nil
}

func (gr *Repo) WriteSummary(s *Summary) error {
	s.GenChecksum()
	if err := s.Validate(); err != nil {
		return err
	}
	slog.Debug("git summary: writing", "file", gr.summaryFile)

	return writeFile(gr.summaryFile, s)
}

func NewRepo(name, dstDir string, opts ...RepoOptFunc) *Repo {
	o := &RepoOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return &Repo{
		name:        name,
		fullpath:    dstDir,
		summaryFile: filepath.Join(dstDir, SummaryFileName),
		RepoOptions: o,
	}
}
