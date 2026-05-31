//go:build ignore

package gitops

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
	gitnew "github.com/mateconpizza/gm/pkg/git"
)

const (
	gpgStrategyName = "gpg"
)

var _ FileReader = (*gitnew.Reader)(nil)

type FileReader interface {
	Read(ctx context.Context, path string)
	Results() ([]*bookmark.Bookmark, error)
}

// RepoConfig groups the configuration needed to read a repository.
type RepoConfig struct {
	root   string
	total  int
	loader *bookio.RepositoryLoader
	sp     *rotato.Rotato
}

// func NewRepoReader(ctx context.Context, app *application.App, total int) ([]*bookmark.Bookmark, error) {
// 	root := app.Path.Git()
//
// 	if gpg.IsInitialized(root) {
// 		return readGPGRepo(ctx, root, nil, total)
// 	}
//
// 	return readJSONRepo(ctx, root, nil, total)
// }

func NewRepoReader(ctx context.Context, app *application.App, total int) FileReader {
	if gpg.IsInitialized(app.Path.Git()) {
		fingerprintPath := gpg.GPGIDPath(app.Git.Path)
		return gpgReader(ctx, fingerprintPath, total)
	}

	return jsonReader(ctx)
}

func readJSONRepo(ctx context.Context, root string, sp *rotato.Rotato, total int) ([]*bookmark.Bookmark, error) {
	return ReadRepo(ctx, RepoConfig{
		root:   root,
		loader: bookio.JSONStrategy,
		sp:     sp,
		total:  total,
	})
}

func readGPGRepo(ctx context.Context, root string, sp *rotato.Rotato, total int) ([]*bookmark.Bookmark, error) {
	return ReadRepo(ctx, RepoConfig{
		root:   root,
		loader: gpgStrategy(root),
		sp:     sp,
		total:  total,
	})
}

// func ReadGPGRepo(ctx context.Context, f *bookio.FileLoader) ([]*bookmark.Bookmark, error) {
// 	g := new(errgroup.Group)
// 	sp := rotato.New(
// 		rotato.WithPrefix("Decrypting GPG bookmarks"),
// 		rotato.WithMessage("waiting for GPG passphrase"),
// 		rotato.WithMessageColor(rotato.StyleDim),
// 		rotato.WithPrefixColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
// 		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
// 	)
//
// 	sp.Start()
//
// 	retrun
// }

// ReadRepo is the unified function that uses the Strategy Pattern.
func ReadRepo(ctx context.Context, cfg RepoConfig) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(cfg.loader.Func, cfg.total)
	if cfg.sp != nil {
		f.WithSpinner(cfg.sp)
	}

	f.Spinner.UpdatePrefix(cfg.loader.Prefix)
	f.Spinner.Start()

	// Only for the GPGStrategy
	var passphrasePrompted bool

	err := filepath.WalkDir(cfg.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, cfg.root, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !cfg.loader.FileFilter(path, d) {
			return nil
		}

		// Handle prompt for GPG passphrase on the first valid file
		if cfg.loader.Name == gpgStrategyName && !passphrasePrompted {
			if err := promptGPGPassphrase(ctx, f, path, &passphrasePrompted); err != nil {
				return err
			}
			passphrasePrompted = true
			return nil
		}

		f.LoadAsync(ctx, path)
		return nil
	})
	if err != nil {
		f.Spinner.Fail(err.Error())
		return nil, err
	}

	return f.Results()
}

func jsonReader(ctx context.Context) FileReader {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(1)

	return &gitnew.Reader{
		Group:  g,
		Mutex:  &sync.Mutex{},
		DoneCh: make(chan struct{}),
		Reader: func(path string) (*bookmark.Bookmark, error) {
			if filepath.Ext(path) != ".json" {
				return nil, gitnew.ErrIgnoreFilepath
			}

			if err := ctx.Err(); err != nil {
				return nil, err
			}

			bj := &bookmark.BookmarkJSON{}
			if err := files.JSONRead(path, bj); err != nil {
				return nil, fmt.Errorf("%w: %s", err, path)
			}
			return bookmark.NewFromJSON(bj), nil
		},
	}
}
