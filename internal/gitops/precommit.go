package gitops

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type (
	// StatsLoader returns the current stats, usually from the database.
	StatsLoader func(ctx context.Context) (*git.RepoStats, error)

	// BookmarksLoader returns the bookmarks to render.
	BookmarksLoader func(ctx context.Context) ([]*bookmark.Bookmark, error)

	// CipherLoader returns the cipher used to write the README.
	CipherLoader func(ctx context.Context) (enc.Cipher, error)
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
func HookReadme(o *ReadmeCfg, bookmarks BookmarksLoader, stats StatsLoader, cipher CipherLoader) *git.Hook {
	return &git.Hook{
		Name: "readme",
		Run: func(ctx context.Context, g *git.Git) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			bs, err := bookmarks(ctx)
			if err != nil {
				return fmt.Errorf("load bookmarks: %w", err)
			}

			fresh, err := stats(ctx)
			if err != nil {
				return fmt.Errorf("load stats: %w", err)
			}

			var buf bytes.Buffer
			if err := NewReadmeFile(&buf, o, fresh, bs); err != nil {
				return fmt.Errorf("generate readme: %w", err)
			}

			c, err := cipher(ctx)
			if err != nil {
				return fmt.Errorf("cipher: %w", err)
			}

			if err := os.MkdirAll(filepath.Dir(o.Path), files.DirPerm); err != nil {
				return err
			}

			// ignore if same file
			fullpath := o.Path + c.Suffix()
			if files.Exists(fullpath) {
				data, err := c.Read(ctx, fullpath)
				if err != nil {
					return err
				}
				if bytes.Equal(data, buf.Bytes()) {
					slog.Debug("pre-commit readme: update-to-date")
					return nil
				}
			}

			g.Log(fmt.Sprintf("git pre-commit: writing README.md (%s)", o.Name))

			return c.Write(ctx, fullpath, buf.Bytes())
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

// RegisterHooks registers the git hooks used to keep repository files up to
// date (summary and README if enabled).
func RegisterHooks(r store, app *application.App, gr *git.Repo) {
	gm := app.Git.Manager()

	hook := gm.Git().PreCommit()
	hook.Reset()

	hook.Register(
		HookSummaryWrite(
			gr,
			app.Version(),
			StatsFromStore(r),
		),
	)

	if app.Git.Readme.Enabled {
		hook.Register(
			HookReadme(
				ReadmeOpts(app, r.BaseName()),
				BookmarksFromStore(r),
				StatsFromStore(r),
				Cipher(app),
			),
		)
	}
}
