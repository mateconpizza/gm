package bookio

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrFileNotFound = errors.New("file not found")

const (
	jsonExt         = ".json"
	summaryFile     = "summary.json"
	trackerFilepath = ".tracked.json"
)

type loadFileFn func(path string) (*bookmark.Bookmark, error)

type BookmarkJSONParser struct{}

// NewJSONParser create a new instance of the Parser.
func NewJSONParser() *BookmarkJSONParser {
	return &BookmarkJSONParser{}
}

// Parse extracts records from a JSON repository.
func (bj *BookmarkJSONParser) Parse(rootPath string) ([]*bookmark.Bookmark, error) {
	var (
		count     uint32
		mu        sync.Mutex
		bookmarks = []*bookmark.Bookmark{}
	)

	loader := func(path string) (*bookmark.Bookmark, error) {
		bj := &bookmark.BookmarkJSON{}
		if err := files.JSONRead(path, bj); err != nil {
			return nil, fmt.Errorf("%w: %s", err, path)
		}

		atomic.AddUint32(&count, 1)
		b := bookmark.NewFromJSON(bj)

		mu.Lock()
		bookmarks = append(bookmarks, b)
		mu.Unlock()

		return b, nil
	}

	// Create errgroup with context
	g, ctx := errgroup.WithContext(context.Background())

	err := filepath.WalkDir(rootPath, parseJSONFile(ctx, g, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	fmt.Printf("Loaded %d bookmarks\n", count)

	return bookmarks, nil
}

// Export creates the repository structure.
func (bj *BookmarkJSONParser) Export(root string, bs []*bookmark.Bookmark) error {
	g := new(errgroup.Group)
	for i := range bs {
		b := bs[i] // capture loop variable
		g.Go(func() error {
			_, err := storeBookmarkAsJSON(root, b, true)
			if err != nil {
				return err
			}

			return nil
		})
	}

	return g.Wait()
}

// storeBookmarkAsJSON creates files structure.
//
//	root -> dbName -> domain -> urlHash.json
//
// Returns true if the file was created or updated, false if no changes were made.
func storeBookmarkAsJSON(rootPath string, b *bookmark.Bookmark, force bool) (bool, error) {
	domain, err := b.Domain()
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// domainPath: root -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// urlHash := domainPath -> urlHash.json
	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+jsonExt)

	updated, err := files.JSONWrite(filePathJSON, b.JSON(), force)
	if err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return updated, nil
}

// resolveFileConflictErr resolves a file conflict error.
// Returns true if the file was updated, false if no update was needed.
func resolveFileConflictErr(
	rootPath string,
	err error,
	filePathJSON string,
	b *bookmark.Bookmark,
) (bool, error) {
	if !errors.Is(err, files.ErrFileExists) {
		return false, err
	}

	bj := bookmark.BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// no need to update
	if bj.Checksum == b.Checksum {
		return false, nil
	}

	return storeBookmarkAsJSON(rootPath, b, true)
}

// parseJSONFile is a WalkDirFunc that loads .json files concurrently using a
// semaphore.
func parseJSONFile(ctx context.Context, g *errgroup.Group, mu *sync.Mutex, l loadFileFn) fs.WalkDirFunc {
	bs := []*bookmark.Bookmark{}
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() ||
			filepath.Ext(path) != jsonExt ||
			filepath.Base(path) == summaryFile ||
			filepath.Base(path) == trackerFilepath {
			return nil
		}

		loadConcurrently(ctx, path, &bs, g, mu, sem, l)
		return nil
	}
}

// loadConcurrently processes a single file in a goroutine using errgroup.
func loadConcurrently(
	ctx context.Context,
	path string,
	bs *[]*bookmark.Bookmark,
	g *errgroup.Group,
	mu *sync.Mutex,
	sem *semaphore.Weighted,
	loader func(path string) (*bookmark.Bookmark, error),
) {
	// Use errgroup.Go instead of manual goroutine + WaitGroup management
	g.Go(func() error {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}
		defer sem.Release(1)

		b, err := loader(path)
		if err != nil {
			return err
		}

		mu.Lock()
		*bs = append(*bs, b)
		mu.Unlock()

		return nil
	})
}
