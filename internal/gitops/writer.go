package gitops

import (
	"context"
	"fmt"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type writerCfg struct {
	name      string // repo name
	root      string // git root path
	repoPath  string // repo fullpath
	bookmarks []*bookmark.Bookmark
	spinner   spinner
	writer    *bookio.RepositoryWriter

	cipher     enc.Cipher
	serializer bookio.Serializer
}

func newFileRepoWriter(ctx context.Context, cfg *writerCfg) error {
	cfg.writer = &bookio.RepositoryWriter{
		Prefix: fmt.Sprintf("[%s] %s", filepath.Base(cfg.repoPath), "Writing bookmarks [%d/%d]"),
		FileFilter: bookio.And(
			bookio.IsFile,
			bookio.NotNamed(git.SummaryFileName, git.TrackerFilename, bookio.ReadmeFile+cfg.cipher.Suffix()),
		),
		Func: func(ctx context.Context, b *bookmark.Bookmark) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			filename, err := cfg.cipher.Path(b, cfg.serializer.Ext())
			if err != nil {
				return err
			}
			path := filepath.Join(cfg.repoPath, filename)
			if err := files.MkdirAll(filepath.Dir(path)); err != nil {
				return fmt.Errorf("creating dir for %s: %w", path, err)
			}

			same, err := hasChecksum(ctx, path, b.Checksum, cfg.cipher, cfg.serializer)
			if err != nil {
				return err
			}
			if same {
				return nil
			}

			data, err := cfg.serializer.Marshal(b.Record())
			if err != nil {
				return fmt.Errorf("marshal %s: %w", path, err)
			}

			cfg.spinner.UpdateMesg("writing..." + filepath.Base(path))

			return cfg.cipher.Write(ctx, path, data)
		},
	}

	cfg.spinner.Start(ctx)
	defer cfg.spinner.Done()

	w := bookio.NewFileWriter(cfg.writer.Func)
	n := len(cfg.bookmarks)

	for i := range n {
		if err := ctx.Err(); err != nil {
			return err
		}

		cfg.spinner.UpdatePrefix(fmt.Sprintf(cfg.writer.Prefix, w.Current(), n))

		b := cfg.bookmarks[i]
		w.Write(ctx, b)
	}

	return w.Wait()
}

// hasChecksum reports whether the file at path exists and holds the given
// checksum.
func hasChecksum(ctx context.Context, path, want string, c enc.Cipher, s bookio.Serializer) (bool, error) {
	if !files.Exists(path) {
		return false, nil
	}

	data, err := c.Read(ctx, path)
	if err != nil {
		return false, err
	}

	r, err := s.Unmarshal(data)
	if err != nil {
		return false, fmt.Errorf("reading existing %s: %w", path, err)
	}

	return r.Checksum == want, nil
}
