package gitops

import (
	"context"
	"log/slog"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

func PreCommitReadme(app *application.App) git.Hook {
	cfg := app.Git.Readme

	return func(ctx context.Context) error {
		if !cfg.Enabled {
			return nil
		}

		root := app.Path.Git()
		path := filepath.Join(root, app.DBBaseName(), bookio.ReadmeFile)

		f, err := files.New(path, true)
		if err != nil {
			return err
		}

		defer func() {
			if err := f.Close(); err != nil {
				slog.Debug("closing destination file", "error", err)
			}
		}()

		r, err := db.New(ctx, app.Path.DB())
		if err != nil {
			return err
		}
		defer r.Close()

		bs, err := r.All(ctx)
		if err != nil {
			return err
		}

		return NewReadmeFile(ctx, app, f, bs)
	}
}
