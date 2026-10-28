package git

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrIgnoreFilepath  = errors.New("git: ignore filepath")
	ErrNoFunctionFound = errors.New("git: no function provided")
	ErrNoStoreFound    = errors.New("git: no store found")
	ErrSummaryFile     = errors.New("git: summary file")
)

type CommitMessage string

func (cm CommitMessage) String() string { return string(cm) }

type RepoAction string

const (
	Add    RepoAction = "add"
	Del    RepoAction = "del"
	Edit   RepoAction = "edit"
	Update RepoAction = "update"
	Rename RepoAction = "rename"
)

type (
	ReaderFunc      func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error)
	WriterFunc      func(ctx context.Context, path string, bs []*bookmark.Bookmark) error
	RemoverFunc     func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error
	PostRemovalFunc func(path string) error
	SumWriterFunc   func(path string, sum *Summary) error

	RepoOptFunc func(*RepoOptions)

	FileWriter[T any] interface {
		Path() string
		Exists() bool
		Write() error
		Read() error
		Data() T
		Set(v T)
	}
)

type RepoOptions struct {
	reader  ReaderFunc
	writer  WriterFunc
	remover RemoverFunc
	file    FileWriter[Summary]
}

func WithRepoWriter(w WriterFunc) RepoOptFunc    { return func(ro *RepoOptions) { ro.writer = w } }
func WithRepoReader(r ReaderFunc) RepoOptFunc    { return func(ro *RepoOptions) { ro.reader = r } }
func WithRepoRemover(rm RemoverFunc) RepoOptFunc { return func(ro *RepoOptions) { ro.remover = rm } }

func WithSummaryWriter[T Summary](sw FileWriter[Summary]) RepoOptFunc {
	return func(ro *RepoOptions) { ro.file = sw }
}

type Repo struct {
	*RepoOptions

	name      string
	fullpath  string
	bookmarks []*bookmark.Bookmark
}

func NewRepo(name, dstDir string, opts ...RepoOptFunc) *Repo {
	sumFilepath := filepath.Join(dstDir, SummaryFileName)
	o := &RepoOptions{
		file: newJSONFile[Summary](sumFilepath),
	}
	for _, opt := range opts {
		opt(o)
	}

	return &Repo{
		name:        strings.ToLower(name),
		fullpath:    dstDir,
		RepoOptions: o,
	}
}

func (gr *Repo) Name() string     { return gr.name }
func (gr *Repo) Root() string     { return filepath.Dir(gr.fullpath) }
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
		return fmt.Errorf("remove bookmark %d from repo %q: %w", b.ID, gr.Fullpath(), err)
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

// Summary returns the current summary from the git repository, loading it
// from disk if present. If no summary file exists yet, it returns a zero
// Summary with the repo name set.
func (gr *Repo) Summary() (*Summary, error) {
	if gr.file.Exists() {
		if err := gr.file.Read(); err != nil {
			return nil, err
		}
	}

	sum := gr.file.Data()
	sum.normalize()
	sum.RepoStats.Name = gr.Name()

	return &sum, nil
}

func (gr *Repo) Update(ctx context.Context, old, fresh *bookmark.Bookmark, postRm PostRemovalFunc) error {
	if err := gr.Rm(ctx, old, postRm); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", old.URL, err)
		}
	}
	return gr.Add(ctx, []*bookmark.Bookmark{fresh})
}

// Stats returns current stats from the git repository.
func (gr *Repo) Stats() (*RepoStats, error) {
	if !gr.file.Exists() {
		return &RepoStats{}, nil
	}

	err := gr.file.Read()
	if err != nil {
		return &RepoStats{}, err
	}

	sum := gr.file.Data()
	sum.RepoStats.Name = gr.Name()
	return sum.RepoStats, nil
}

func (gr *Repo) WriteSummary(s *Summary) error {
	s.GenChecksum()
	if err := s.Validate(); err != nil {
		return err
	}
	slog.Debug("git summary: writing", "file", gr.file.Path())
	s.RepoStats.Name = gr.Name()
	gr.file.Set(*s)
	return gr.file.Write()
}

// CommitMsg construct the repo commit message.
//
//	[repoName] action object
func (gr *Repo) CommitMsg(a RepoAction, obj string) CommitMessage {
	return CommitMessage(fmt.Sprintf("[%s] %s %s", gr.Name(), a, obj))
}
