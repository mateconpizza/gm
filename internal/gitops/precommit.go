package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type (
	// StatsLoader returns the current stats, usually from the database.
	StatsLoader func(ctx context.Context) (*git.RepoStats, error)

	// BookmarksLoader returns the bookmarks to render.
	BookmarksLoader func(ctx context.Context) ([]*bookmark.Bookmark, error)
)

// HookSummaryWrite creates a hook that updates the repository summary.
func HookSummaryWrite(gr *git.Repo, version string, load StatsLoader) *git.Hook {
	return &git.Hook{
		Name: "summary",
		Run: func(ctx context.Context, g *git.Git) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			fresh, err := load(ctx)
			if err != nil {
				return fmt.Errorf("fresh stats: %w", err)
			}

			hasChanges, err := g.HasChanges(ctx)
			if err != nil {
				return err
			}

			old, err := gr.Summary()
			if err != nil {
				return fmt.Errorf("read summary: %w", err)
			}

			// Skip writing if neither git workspace nor database stats changed
			if !hasChanges && old.RepoStats.Equal(fresh) {
				return git.ErrGitUpToDate
			}

			hostname, err := os.Hostname()
			if err != nil {
				return err
			}

			sum, err := git.BuildSummary(ctx, g, fresh, hostname, version)
			if err != nil {
				return err
			}

			if err := sum.Validate(); err != nil {
				return err
			}

			g.Log(fmt.Sprintf("git pre-commit: writing summary (%s)", gr.Name()))

			return gr.WriteSummary(sum)
		},
	}
}

// HookReadme creates a Git hook that generates and writes the README.md.
func HookReadme(app *application.App, bookmarks BookmarksLoader, stats StatsLoader) *git.Hook {
	return &git.Hook{
		Name: "readme",
		Run: func(ctx context.Context, g *git.Git) error {
			if !app.Git.Readme.Enabled {
				return nil
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			path := app.Path.GitReadme(app.DBBaseName())

			f, err := files.New(path, true)
			if err != nil {
				return err
			}

			defer func() {
				if err := f.Close(); err != nil {
					slog.Debug("closing destination file", "error", err)
				}
			}()

			bs, err := bookmarks(ctx)
			if err != nil {
				return fmt.Errorf("load bookmarks: %w", err)
			}

			g.Log(fmt.Sprintf("git pre-commit: writing README.md (%s)", app.DBBaseName()))

			fresh, err := stats(ctx)
			if err != nil {
				return err
			}

			return NewReadmeFile(ctx, f, app, fresh, bs)
		},
	}
}

func HookHasChanges() *git.Hook {
	return &git.Hook{
		Name: "changes",
		Run: func(ctx context.Context, g *git.Git) error {
			changes, err := g.HasChanges(ctx)
			if err != nil {
				return err
			}
			if !changes {
				return git.ErrGitUpToDate
			}
			return nil
		},
	}
}
