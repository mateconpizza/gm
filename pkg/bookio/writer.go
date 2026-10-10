package bookio

import (
	"context"
	"runtime"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type WriterFileFunc func(ctx context.Context, b *bookmark.Bookmark) error

type FileWriter struct {
	current atomic.Uint32
	g       *errgroup.Group
	writer  WriterFileFunc
}

// NewFileWriter creates a concurrent file writer with a CPU-sized worker
// limit.
func NewFileWriter(writer WriterFileFunc) *FileWriter {
	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU())

	return &FileWriter{
		g:      g,
		writer: writer,
	}
}

// Write writes a bookmark asynchronously to the given path.
func (f *FileWriter) Write(ctx context.Context, b *bookmark.Bookmark) {
	f.g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := f.writer(ctx, b); err != nil {
				return err
			}

			f.Count(1)
			return nil
		}
	})
}

// Wait waits for all writes to complete.
func (f *FileWriter) Wait() error {
	return f.g.Wait()
}

// Count increments the processed item count and returns the new value.
func (f *FileWriter) Count(n uint32) uint32 {
	return f.current.Add(n)
}

// Current returns the current number of processed items.
func (f *FileWriter) Current() uint32 {
	return f.current.Load()
}

// RepositoryWriter configures how a repository is read.
type RepositoryWriter struct {
	Func       WriterFileFunc
	Prefix     string
	FileFilter FileFilterFunc
}
