package gitops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
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

// readerCfg groups the configuration needed to read a repository.
type readerCfg struct {
	name     string // repo name
	root     string // git root path
	repoPath string // repo fullpath
	total    int    // total bookmarks
	loader   *bookio.RepositoryLoader
	spinner  spinner

	promptPassphrase PassphrasePrompt

	cipher     enc.Cipher
	serializer bookio.Serializer
}

func (c *readerCfg) passphrasePrompt() PassphrasePrompt {
	if c.promptPassphrase != nil {
		return c.promptPassphrase
	}
	return promptGPGPassphrase
}

func fileReader(cfg *readerCfg) *bookio.RepositoryLoader {
	filters := make([]bookio.FileFilterFunc, 0, 3)
	ext := cfg.cipher.Suffix()
	if ext != "" {
		filters = append(filters, bookio.HasExtension(cfg.cipher.Suffix()))
	}

	filters = append(filters,
		bookio.IsFile,
		bookio.NotNamed(git.SummaryFileName, git.TrackerFilename, bookio.ReadmeFile+cfg.cipher.Suffix()),
	)

	return &bookio.RepositoryLoader{
		Prefix:     fmt.Sprintf("[%s] %s", filepath.Base(cfg.repoPath), "bookmarks"),
		FileFilter: bookio.And(filters...),
		Func: func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			data, err := cfg.cipher.Read(ctx, path)
			if err != nil {
				return nil, err
			}

			br, err := cfg.serializer.Unmarshal(data)
			if err != nil {
				return nil, fmt.Errorf("%w path: %q", err, path)
			}

			return br.Bookmark(), nil
		},
	}
}

func newFileRepoReader(ctx context.Context, cfg *readerCfg) ([]*bookmark.Bookmark, error) {
	cfg.loader = fileReader(cfg)
	cfg.spinner.UpdatePrefix(cfg.loader.Prefix)
	cfg.spinner.Start(ctx)
	defer cfg.spinner.Done()

	r := bookio.NewFileLoader(cfg.loader.Func)

	var passphrasePrompted bool

	if err := filepath.WalkDir(cfg.repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, cfg.root, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !cfg.loader.FileFilter(path, d) {
			return nil
		}

		// handle prompt for GPG passphrase on the first valid file.
		if !passphrasePrompted {
			if err := cfg.passphrasePrompt()(ctx, r, cfg.spinner, path, &passphrasePrompted); err != nil {
				return err
			}
			passphrasePrompted = true
		}

		cfg.spinner.UpdatePrefix(fmt.Sprintf("%s [%d/%d]", cfg.loader.Prefix, r.Count(1), cfg.total))
		cfg.spinner.UpdateMesg("reading..." + filepath.Base(path))

		r.Load(ctx, path)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrRepoReader, cfg.name, err)
	}

	return r.Results()
}
