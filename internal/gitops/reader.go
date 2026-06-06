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

// RepoReaderCfg groups the configuration needed to read a repository.
type RepoReaderCfg struct {
	root   string
	loader *bookio.RepositoryLoader
	sp     *rotato.Rotato
	total  int
}

func NewRepoReader(ctx context.Context, gitRoot, repoPath string, n int) ([]*bookmark.Bookmark, error) {
	boldRed := rotato.FgBrightRed.With(rotato.StyleBold)

	sp := rotato.New(
		rotato.WithMessage("starting..."),
		rotato.WithPrefixColor(rotato.StyleDim),
		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
		rotato.WithMessageColor(rotato.FgBrightBlue.With(rotato.StyleItalic)),
		rotato.WithFailSymbolColor(boldRed),
		rotato.WithFailMessageColor(boldRed),
	)

	if gpg.IsInitialized(gitRoot) {
		fingerprintPath := gpg.GPGIDPath(gitRoot)
		fp, err := gpg.LookupKey(fingerprintPath)
		if err != nil {
			return nil, err
		}

		if fp.Expired() {
			sp.AddPrefixDecorator(func(mesg string) string {
				return mesg + rotato.FgBrightYellow.Wrap(" warn: key has expired", rotato.StyleItalic)
			})
		}

		loader, err := gpgStrategy(fp.Fingerprint)
		if err != nil {
			return nil, err
		}

		return ReadGPGRepo(ctx, RepoReaderCfg{
			root:   repoPath,
			loader: loader,
			sp:     sp,
			total:  n,
		})
	}

	return ReadJSONRepo(ctx, RepoReaderCfg{
		root:   repoPath,
		loader: bookio.JSONStrategy,
		sp:     sp,
		total:  n,
	})
}

// ReadJSONRepo handles reading standard JSON bookmark repositories.
func ReadJSONRepo(ctx context.Context, cfg RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(cfg.loader.Func)

	cfg.sp.UpdatePrefix(cfg.loader.Prefix)
	cfg.sp.Start(ctx)
	defer cfg.sp.Done()

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

		cfg.sp.UpdatePrefix(fmt.Sprintf("%s [%d/%d]", cfg.loader.Prefix, f.Count(1), cfg.total))
		cfg.sp.UpdateMesg("reading..." + filepath.Base(path))

		f.LoadAsync(ctx, path)

		return nil
	}); err != nil {
		cfg.sp.Fail(err.Error())
		return nil, err
	}

	return f.Results()
}
