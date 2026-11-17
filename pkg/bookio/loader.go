package bookio

import (
	"context"
	"log/slog"
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
	logger  *slog.Logger
}

// NewFileLoader creates a concurrent file loader with a CPU-sized worker
// limit.
func NewFileLoader(loader LoaderFileFunc) *FileLoader {
	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU())

	return &FileLoader{
		g:       g,
		Loader:  loader,
		mu:      &sync.Mutex{},
		results: make([]*bookmark.Bookmark, 0),
		logger:  slog.New(slog.DiscardHandler),
	}
}

func (f *FileLoader) WithLogger(l *slog.Logger) *FileLoader {
	if l != nil {
		f.logger = l.With("cmp", "file-reader")
	}
	return f
}

// Load loads a bookmark asynchronously from the given path.
func (f *FileLoader) Load(ctx context.Context, path string) {
	log := f.logger.With("fn", "load", "path", path)

	f.g.Go(func() error {
		select {
		case <-ctx.Done():
			log.Debug("bookmark load canceled", "error", ctx.Err())
			return ctx.Err()
		default:
		}

		b, err := f.Loader(ctx, path)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Debug("bookmark load canceled", "error", ctxErr)
			} else {
				log.Error("failed to load bookmark", "error", err)
			}
			return err
		}

		f.add(b)
		count := f.Count(1)

		log.Debug("bookmark loaded", "count", count)

		return nil
	})
}

// Results waits for all loads to complete and returns the results.
func (f *FileLoader) Results() ([]*bookmark.Bookmark, error) {
	log := f.logger.With("fn", "results")

	if err := f.g.Wait(); err != nil {
		log.Debug("bookmark loading stopped",
			"loaded", f.current.Load(),
			"error", err,
		)
		return nil, err
	}

	log.Debug("all bookmark loads completed", "count", f.current.Load())

	return f.results, nil
}

// Count increments the processed item count and returns the new value.
func (f *FileLoader) Count(n uint32) uint32 { return f.current.Add(n) }

// Current returns the current number of processed items.
func (f *FileLoader) Current() uint32 { return f.current.Load() }

// add appends a bookmark to the result set.
func (f *FileLoader) add(b *bookmark.Bookmark) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, b)
}

// RepositoryLoader configures how a repository is loaded.
type RepositoryLoader struct {
	Func       LoaderFileFunc
	Prefix     string
	FileFilter FileFilterFunc
}
