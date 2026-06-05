package bookio

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type LoaderFileFunc func(ctx context.Context, path string) (*bookmark.Bookmark, error)

type FileLoader struct {
	current atomic.Uint32
	g       *errgroup.Group
	mu      *sync.Mutex
	results []*bookmark.Bookmark
	Loader  LoaderFileFunc
}

// LoadAsync loads a bookmark asynchronously from the given path.
func (f *FileLoader) LoadAsync(ctx context.Context, path string) {
	f.g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			b, err := f.Loader(ctx, path)
			if err != nil {
				return err
			}

			f.add(b)

			return nil
		}
	})
}

// add appends a bookmark to the result set.
func (f *FileLoader) add(b *bookmark.Bookmark) {
	f.mu.Lock()
	f.results = append(f.results, b)
	f.mu.Unlock()
}

// Results waits for all loads to complete and returns the results.
func (f *FileLoader) Results() ([]*bookmark.Bookmark, error) {
	if err := f.g.Wait(); err != nil {
		return nil, err
	}

	return f.results, nil
}

// Count increments the processed item count and returns the new value.
func (f *FileLoader) Count(n uint32) uint32 {
	return f.current.Add(n)
}

// RepositoryLoader configures how a repository is loaded.
type RepositoryLoader struct {
	Func       LoaderFileFunc
	Prefix     string
	FileFilter FileFilterFunc
}

// NewFileLoader creates a concurrent file loader with a CPU-sized worker
// limit.
func NewFileLoader(loader LoaderFileFunc) *FileLoader {
	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU())

	return &FileLoader{
		g:      g,
		Loader: loader,
		mu:     &sync.Mutex{},
	}
}
