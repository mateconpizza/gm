package bookio

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type loadFileFn func(path string) (*bookmark.Bookmark, error)

type FileLoader struct {
	ctx     context.Context
	count   uint32
	g       *errgroup.Group
	mu      *sync.Mutex
	sem     *semaphore.Weighted
	results []*bookmark.Bookmark
	Loader  loadFileFn
	Spinner *rotato.Rotato
}

func (f *FileLoader) WithLoader(loader func(string) (*bookmark.Bookmark, error)) *FileLoader {
	f.Loader = loader
	return f
}

func (f *FileLoader) WithSpinner(sp *rotato.Rotato) *FileLoader {
	f.Spinner = sp
	return f
}

func (f *FileLoader) LoadAsync(path string) {
	f.g.Go(func() error {
		if err := f.sem.Acquire(f.ctx, 1); err != nil {
			return err
		}
		defer f.sem.Release(1)

		b, err := f.Loader(path)
		currentCount := atomic.AddUint32(&f.count, 1)
		f.Spinner.UpdateMesg(fmt.Sprintf("[%d] %s", currentCount, filepath.Base(path)))
		if err != nil {
			return err
		}

		f.add(b)

		return nil
	})
}

func (f *FileLoader) add(b *bookmark.Bookmark) {
	f.mu.Lock()
	f.results = append(f.results, b)
	f.mu.Unlock()
}

func (f *FileLoader) Results() ([]*bookmark.Bookmark, error) {
	if err := f.g.Wait(); err != nil {
		return nil, err
	}

	f.Spinner.UpdatePrefix("")
	f.Spinner.Done(fmt.Sprintf("found %d bookmarks", f.count))

	return f.results, nil
}

func NewFileLoader(ctx context.Context) *FileLoader {
	g, ctx := errgroup.WithContext(ctx)
	return &FileLoader{
		ctx: ctx,
		g:   g,
		mu:  &sync.Mutex{},
		sem: semaphore.NewWeighted(1),
		Spinner: rotato.New(
			rotato.WithMesgColor(rotato.ColorBrightBlue),
			rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
			rotato.WithFailColorMesg(rotato.ColorBrightRed, rotato.ColorStyleItalic),
		),
	}
}

// RepositoryLoader defines the strategy for loading a specific type of
// repository.
type RepositoryLoader struct {
	LoaderFn   loadFileFn
	Prefix     string
	FileFilter FileFilterFunc
}
