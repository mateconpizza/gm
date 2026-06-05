package bookio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrFileRemoverRootEmpty  = errors.New("root path cannot be empty")
	ErrFileMgrNil            = errors.New("file manager cannot be nil")
	ErrFileRemoveGenFullpath = errors.New("genFullpath function cannot be nil")
)

type FileManager interface {
	ExistsErr(path string) error
	Rm(path string) error
	RmEmptyDirs(root string) error
}

type GenFullpathFn func(root string, b *bookmark.Bookmark) (string, error)

type FileRemover struct {
	root        string
	fm          FileManager
	genFullpath GenFullpathFn
}

func (fr *FileRemover) Rm(ctx context.Context, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for i := range bs {
		b := bs[i]

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				fullpath, err := fr.genFullpath(fr.root, b)
				if err != nil {
					return fmt.Errorf("generating fullpath for %d: %w", b.ID, err)
				}

				if err := fr.fm.ExistsErr(fullpath); err != nil {
					return fmt.Errorf("checking file %s: %w", fullpath, err)
				}

				if err := fr.fm.Rm(fullpath); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return nil
					}
					return fmt.Errorf("removing %s: %w", fullpath, err)
				}
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Clean up empty directories
	if err := fr.fm.RmEmptyDirs(fr.root); err != nil {
		return fmt.Errorf("removing empty dirs: %w", err)
	}

	return nil
}

func NewFileRemover(root string, fm FileManager, genFullpath GenFullpathFn) (*FileRemover, error) {
	if root == "" {
		return nil, ErrFileRemoverRootEmpty
	}

	if fm == nil {
		return nil, ErrFileMgrNil
	}

	if genFullpath == nil {
		return nil, ErrFileRemoveGenFullpath
	}

	return &FileRemover{
		root:        root,
		genFullpath: genFullpath,
		fm:          fm,
	}, nil
}
