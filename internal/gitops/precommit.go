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
func HookSummaryWrite(gr *git.Repo, version string, load StatsLoader, l *slog.Logger) *git.Hook {
	log := l.With("hook", "summary", "repo", gr.Name())

	return &git.Hook{
		Name: "summary",
		Run: func(ctx context.Context, g *git.Git) error {
			if err := ctx.Err(); err != nil {
				log.Debug("context done before hook", "cause", context.Cause(ctx))
				return err
			}

			log.Debug("checking summary")

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

			statsEqual := old.RepoStats.Equal(fresh)
			log.Debug("compared state", "workspace_changes", hasChanges, "stats_equal", statsEqual)

			// Skip writing if neither git workspace nor database stats changed
			if !hasChanges && statsEqual {
				log.Debug("summary up to date, skipping")
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

			if err := gr.WriteSummary(sum); err != nil {
				return err
			}

			log.Debug("summary written", "version", version)

			return nil
		},
	}
}

// HookReadme creates a Git hook that generates and writes the README.md.
func HookReadme(o *ReadmeCfg, bookmarks BookmarksLoader, stats StatsLoader, cipher CipherLoader, l *slog.Logger) *git.Hook {
	log := l.With("hook", "readme", "repo", o.Name)

	return &git.Hook{
		Name: "readme",
		Run: func(ctx context.Context, g *git.Git) error {
			if err := ctx.Err(); err != nil {
				log.Debug("context done before hook", "cause", context.Cause(ctx))
				return err
			}

			log.Debug("checking readme")

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
			log.Debug("readme generated", "bookmarks", len(bs), "bytes", buf.Len())

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
					log.Debug("readme up to date, skipping", "file", filepath.Base(fullpath))
					return nil
				}
				log.Debug("readme differs from existing file", "file", filepath.Base(fullpath), "old_bytes", len(data), "new_bytes", buf.Len())
			} else {
				log.Debug("readme file does not exist yet", "file", filepath.Base(fullpath))
			}

			g.Log(fmt.Sprintf("git pre-commit: writing README.md (%s)", o.Name))

			if err := c.Write(ctx, fullpath, buf.Bytes()); err != nil {
				return err
			}

			log.Debug("readme written", "file", filepath.Base(fullpath), "bytes", buf.Len())

			return nil
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
func RegisterHooks(r store, app *application.App, gm *git.Mgr, gr *git.Repo) {
	hook := gm.Git().PreCommit()
	hook.Reset()

	hook.Register(
		HookSummaryWrite(
			gr,
			app.Version(),
			StatsFromStore(r),
			app.Logger(),
		),
	)

	if app.Git.Readme.Enabled {
		hook.Register(
			HookReadme(
				ReadmeOpts(app, r.BaseName()),
				BookmarksFromStore(r),
				StatsFromStore(r),
				Cipher(app),
				app.Logger(),
			),
		)
	}
}
