package gitops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrRepoReader = errors.New("reading repository")

type spinner interface {
	Start(ctx context.Context)
	Done(mesg ...string)
	Fail(mesg ...string)

	AddPrefixDecorator(fn func(mesg string) string)
	SetMessageDecorator(fn func(mesg string) string)
	UpdateMesg(s string)
	UpdatePrefix(s string)
}

// PassphrasePrompt prompts for (and caches) the GPG passphrase before
// decrypting repository files.
type PassphrasePrompt func(ctx context.Context, f *bookio.FileLoader, sp spinner, path string, done *bool) error

// RepoReaderCfg groups the configuration needed to read a repository.
type RepoReaderCfg struct {
	name     string // repo name
	root     string // git root path
	fullpath string // repo fullpath
	total    int    // total bookmarks
	loader   *bookio.RepositoryLoader
	spinner  spinner

	promptPassphrase PassphrasePrompt
}

func (c *RepoReaderCfg) passphrasePrompt() PassphrasePrompt {
	if c.promptPassphrase != nil {
		return c.promptPassphrase
	}
	return promptGPGPassphrase
}

func newRepoReader(ctx context.Context, cfg *RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	switch {
	case gpg.IsInitialized(cfg.root):
		return readGPGRepo(ctx, cfg)

	default:
		return readJSONRepo(ctx, cfg)
	}
}

// ReadRepoFiles handles reading standard JSON bookmark repositories.
func ReadRepoFiles(ctx context.Context, cfg *RepoReaderCfg) ([]*bookmark.Bookmark, error) {
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
		return nil, fmt.Errorf("%w: %w", ErrRepoReader, err)
	}

	return f.Results()
}

// readJSONRepo reads a plain, unencrypted JSON repository.
func readJSONRepo(ctx context.Context, cfg *RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	cfg.loader = bookio.JSONStrategy
	return ReadRepoFiles(ctx, cfg)
}
