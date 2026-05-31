//go:build ignore

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

const (
	JSONFileExt = ".json"
	GPGFileExt  = ".gpg"
)

type FileReader interface {
	Read(ctx context.Context, path string) error
	Results() ([]*bookmark.Bookmark, error)
}

// type FileLoader interface {
// 	Load(ctx context.Context, path string)
// 	Results() ([]*bookmark.Bookmark, error)
// }

// var _ FileReader = (*Reader)(nil)
//
//	type FileReader interface {
//		Read(ctx context.Context, path string)
//		Results() ([]*bookmark.Bookmark, error)
//	}
type Reader struct {
	Group   *errgroup.Group
	Mutex   *sync.Mutex
	results []*bookmark.Bookmark
	Reader  ReaderFunc

	DoneCh chan struct{}
}

func (r *Reader) add(b *bookmark.Bookmark) {
	r.Mutex.Lock()
	r.results = append(r.results, b)
	r.Mutex.Unlock()
}

func (r *Reader) Read(ctx context.Context, path string) {
	r.Group.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b, err := r.Reader(path)
		if err != nil {
			if errors.Is(err, ErrIgnoreFilepath) {
				return nil
			}
			return err
		}

		r.add(b)
		return nil
	})
}

func (r *Reader) Results() ([]*bookmark.Bookmark, error) {
	defer close(r.DoneCh)

	if err := r.Group.Wait(); err != nil {
		return nil, err
	}

	return r.results, nil
}

func reader(ctx context.Context, srcDir string, fr FileReader) ([]*bookmark.Bookmark, error) {
	if !fileExists(srcDir) {
		return nil, fmt.Errorf("%w: %q", os.ErrNotExist, srcDir)
	}

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, srcDir, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Base(path) == "summary.json" || filepath.Base(path) == ".tracked.json" {
			return nil
		}

		fr.Read(ctx, path)

		return nil
	})
	if err != nil {
		return nil, err
	}

	bs, err := fr.Results()
	if err != nil {
		return nil, err
	}

	return bs, nil
}
