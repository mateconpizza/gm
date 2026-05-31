package git

import (
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
	ReaderFn  func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error)
	WriterFn  func(ctx context.Context, path string, b *bookmark.Bookmark) error
	RemoverFn func(ctx context.Context, repoPath string, b *bookmark.Bookmark) error
)

type RepoOptFunc func(*RepoOptions)

type RepoOptions struct {
	reader  ReaderFn
	writer  WriterFn
	remover RemoverFn
	db      RepoDB
}

func WithRepoWriter(w WriterFn) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.writer = w
	}
}

func WithRepoReader(r ReaderFn) RepoOptFunc {
	return func(ro *RepoOptions) {
		ro.reader = r
	}
}

func WithRepoRemover(rm RemoverFn) RepoOptFunc {
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

func (gr *Repo) Name() string { return gr.name }

func (gr *Repo) Root() string { return filepath.Dir(gr.fullpath) }

func (gr *Repo) DB() RepoDB { return gr.db }

func (gr *Repo) Fullpath() string { return gr.fullpath }

func (gr *Repo) String() string {
	stats, err := gr.Stats()
	if err != nil {
		slog.Error("error getting repo summary", "error", err)
		return ""
	}
	return stats.String()
}

func (gr *Repo) Bookmarks() []*bookmark.Bookmark { return gr.bookmarks }

func (gr *Repo) Add(ctx context.Context, bs []*bookmark.Bookmark) error {
	if gr.writer == nil {
		return fmt.Errorf("%w: file writer", ErrNoFunctionFound)
	}

	for i := range bs {
		b := bs[i]
		if err := gr.writer(ctx, gr.fullpath, b); err != nil {
			return err
		}

		gr.bookmarks = append(gr.bookmarks, b)
	}

	return nil
}

func (gr *Repo) RmMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	for i := range bs {
		b := bs[i]
		if err := gr.Rm(ctx, b); err != nil {
			return err
		}
	}

	return nil
}

func (gr *Repo) Rm(ctx context.Context, b *bookmark.Bookmark) error {
	if gr.remover == nil {
		return fmt.Errorf("%w: file remover", ErrNoFunctionFound)
	}
	if err := gr.remover(ctx, gr.Fullpath(), b); err != nil {
		return err
	}

	gr.bookmarks = slices.DeleteFunc(
		gr.bookmarks,
		func(e *bookmark.Bookmark) bool {
			return e.ID == b.ID
		},
	)
	return nil
}

func (gr *Repo) Read(ctx context.Context, total int) error {
	if gr.reader == nil {
		return fmt.Errorf("%w: file reader", ErrNoFunctionFound)
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
