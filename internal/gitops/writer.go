package gitops

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type RepoWriterCfg struct {
	name      string // repo name
	root      string // git root path
	repoPath  string // repo fullpath
	bookmarks []*bookmark.Bookmark
	spinner   spinner
	writer    *bookio.RepositoryWriter
}

func newRepoWriter(ctx context.Context, cfg *RepoWriterCfg) error {
	switch {
	case gpg.IsInitialized(cfg.root):
		return newGPGRepoWriter(ctx, cfg)

	default:
		return newJSONRepoWriter(ctx, cfg)
	}
}

func writeFiles(color bool) func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
	sp := rotato.New(
		rotato.WithColor(color),
		rotato.WithMessage("starting..."),
		rotato.WithPrefixColor(rotato.StyleDim),
		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
		rotato.WithMessageColor(rotato.FgBrightBlue.With(rotato.StyleItalic)),
		rotato.WithFailSymbolColor(rotato.FgBrightRed.With(rotato.StyleBold)),
		rotato.WithFailMessageColor(rotato.FgBrightRed.With(rotato.StyleBold)),
	)

	return func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
		return newRepoWriter(ctx, &RepoWriterCfg{
			name:      filepath.Base(path),
			root:      filepath.Dir(path),
			repoPath:  path,
			bookmarks: bs,
			spinner:   sp,
		})
	}
}

func newGPGRepoWriter(ctx context.Context, cfg *RepoWriterCfg) error {
	k := gpg.NewKeyResolver(cfg.root)
	fp, err := k.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("gpg strategy: %w", err)
	}

	if err := fp.Validate(); err != nil {
		return err
	}

	cfg.spinner.Start(ctx)
	defer cfg.spinner.Done()

	n := len(cfg.bookmarks)
	t := terminal.New()
	g := gpg.New(fp.Fingerprint)

	cfg.writer = WriterGPGStrategy(cfg.repoPath, g)
	writer := bookio.NewFileWriter(cfg.writer.Func)

	for i := range n {
		if err := ctx.Err(); err != nil {
			return err
		}

		b := cfg.bookmarks[i]
		writer.Write(ctx, b)

		cfg.spinner.UpdatePrefix(fmt.Sprintf(cfg.writer.Prefix, writer.Current(), n))
		cfg.spinner.UpdateMesg("encrypting..." + filepath.Base(txt.Shorten(b.URL, t.MinWidth())))
	}

	return nil
}

func newJSONRepoWriter(ctx context.Context, cfg *RepoWriterCfg) error {
	cfg.writer = WriterJSONStrategy(cfg.repoPath)
	cfg.spinner.Start(ctx)
	defer cfg.spinner.Done()

	writer := bookio.NewFileWriter(cfg.writer.Func)

	for i := range cfg.bookmarks {
		if err := ctx.Err(); err != nil {
			return err
		}

		b := cfg.bookmarks[i]
		writer.Write(ctx, b)
	}

	return writer.Wait()
}

func WriterJSONStrategy(repoPath string) *bookio.RepositoryWriter {
	return &bookio.RepositoryWriter{
		Prefix: "Writing JSON bookmarks",
		FileFilter: bookio.And(
			bookio.IsFile,
			bookio.HasExtension(".json"),
			bookio.NotNamed(git.SummaryFileName, git.TrackerFile),
		),
		Func: func(ctx context.Context, b *bookmark.Bookmark) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := bookio.SaveAsJSON(repoPath, b, true)
			return err
		},
	}
}

func WriterGPGStrategy(repoPath string, g Encryptor) *bookio.RepositoryWriter {
	name := filepath.Base(repoPath)

	return &bookio.RepositoryWriter{
		Prefix: fmt.Sprintf("[%s] %s", name, "Encrypting bookmarks [%d/%d]"),
		FileFilter: bookio.And(
			bookio.IsFile,
			bookio.HasExtension(gpg.Extension.String()),
			bookio.NotNamed(git.SummaryFileName),
		),
		Func: func(ctx context.Context, b *bookmark.Bookmark) error {
			return createGPGFile(ctx, g, repoPath, b)
		},
	}
}
