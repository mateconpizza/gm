package bookio

import (
	"context"
	"log/slog"
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
	logger  *slog.Logger
}

// NewFileWriter creates a concurrent file writer with a CPU-sized worker
// limit.
func NewFileWriter(writer WriterFileFunc) *FileWriter {
	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU())

	return &FileWriter{
		g:      g,
		writer: writer,
		logger: slog.New(slog.DiscardHandler),
	}
}

func (f *FileWriter) WithLogger(l *slog.Logger) *FileWriter {
	if l != nil {
		f.logger = l.With("cmp", "file-writer")
	}
	return f
}

// Write writes a bookmark asynchronously to the given path.
func (f *FileWriter) Write(ctx context.Context, b *bookmark.Bookmark) {
	log := f.logger.With("fn", "write")

	f.g.Go(func() error {
		select {
		case <-ctx.Done():
			log.Debug("bookmark write canceled", "error", ctx.Err())
			return ctx.Err()
		default:
		}

		if err := f.writer(ctx, b); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Debug("bookmark write canceled", "error", ctxErr)
			} else {
				log.Error("failed to write bookmark", "error", err)
			}

			return err
		}

		count := f.Count(1)

		log.Debug("bookmark written", "count", count)

		return nil
	})
}

// Wait waits for all writes to complete.
func (f *FileWriter) Wait() error {
	log := f.logger.With("fn", "wait")

	if err := f.g.Wait(); err != nil {
		return err
	}

	log.Debug("all bookmark writes completed", "count", f.Current())

	return nil
}

// Count increments the processed item count and returns the new value.
func (f *FileWriter) Count(n uint32) uint32 { return f.current.Add(n) }

// Current returns the current number of processed items.
func (f *FileWriter) Current() uint32 { return f.current.Load() }

// RepositoryWriter configures how a repository is read.
type RepositoryWriter struct {
	Func       WriterFileFunc
	Prefix     string
	FileFilter FileFilterFunc
}
