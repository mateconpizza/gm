package gitops

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type spinner interface {
	Start(ctx context.Context)
	Done(mesg ...string)
	Fail(mesg ...string)

	AddPrefixDecorator(fn rotato.MessageDecorator)
	SetMessageDecorator(fn rotato.MessageDecorator)
	UpdateMesg(s string)
	UpdatePrefix(s string)
}

// RepoReaderCfg groups the configuration needed to read a repository.
type RepoReaderCfg struct {
	name     string // repo name
	root     string // git root path
	fullpath string // repo fullpath
	total    int    // total bookmarks
	loader   *bookio.RepositoryLoader
	spinner  spinner
}

func newRepoReader(ctx context.Context, opts *RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	if gpg.IsInitialized(opts.root) {
		fingerprintPath := gpg.GPGIDPath(opts.root)
		fp, err := gpg.LookupKey(ctx, fingerprintPath)
		if err != nil {
			return nil, err
		}

		if fp.Expired() {
			opts.spinner.AddPrefixDecorator(func(mesg string) string {
				return mesg + rotato.FgBrightYellow.Wrap(" warn: key has expired", rotato.StyleItalic)
			})
		}

		loader, err := gpgStrategy(opts.name, fp.Fingerprint)
		if err != nil {
			return nil, err
		}
		opts.loader = loader

		return ReadGPGRepo(ctx, opts)
	}

	opts.loader = bookio.JSONStrategy
	return ReadJSONRepo(ctx, opts)
}

// ReadJSONRepo handles reading standard JSON bookmark repositories.
func ReadJSONRepo(ctx context.Context, cfg *RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(cfg.loader.Func)

	cfg.spinner.UpdatePrefix(cfg.loader.Prefix)
	cfg.spinner.Start(ctx)
	defer cfg.spinner.Done()

	if err := filepath.WalkDir(cfg.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, cfg.root, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !cfg.loader.FileFilter(path, d) {
			return nil
		}

		cfg.spinner.UpdatePrefix(fmt.Sprintf("%s [%d/%d]", cfg.loader.Prefix, f.Count(1), cfg.total))
		cfg.spinner.UpdateMesg("reading..." + filepath.Base(path))

		f.LoadAsync(ctx, path)

		return nil
	}); err != nil {
		cfg.spinner.Fail(err.Error())
		return nil, err
	}

	return f.Results()
}
