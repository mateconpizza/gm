package gitops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

func NewManager(app *application.App) (*git.Mgr, error) {
	g, err := NewGit(app)
	if err != nil {
		return nil, err
	}

	return git.NewManager(
		app.Path.Git(),
		git.WithGit(g),
		git.WithVersion(app.Version()),
	)
}

func NewRepo(m *git.Mgr, name string, opts ...git.RepoOptFunc) *git.Repo {
	opts = append(
		opts,
		RepoFileReader(),
		RepoFileRemover(),
		RepoFileWriter(),
	)

	return m.NewRepo(name, opts...)
}

func NewGit(app *application.App) (*git.Git, error) {
	return git.New(
		app.Path.Git(),
		[]git.GitOpt{
			// add Command logger
			git.WithGitCommandLogger(func(w io.Writer, commands []string) {
				headerFrame := frame.New(
					frame.WithColorBorder(ansi.BrightYellow),
					frame.WithBordersSmallBlock(),
					frame.WithWriter(w),
				)
				fullCmd := ansi.BrightYellow.Wrap(strings.Join(commands, " "), ansi.Italic)
				headerFrame.Midln(fullCmd).Flush()
			}),

			// writer
			git.WithGitWriter(app.Git.Writer()),
		}...,
	)
}

func Add(ctx context.Context, app *application.App, r *db.SQLite, b *bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	m, err := NewManager(app)
	if err != nil {
		return err
	}

	name := r.BaseName()
	if !m.IsEnabled() || !m.IsTracked(name) {
		return nil
	}

	gr := NewRepo(m, r.Name(), git.WithRepoStore(r))
	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}

	return m.SaveChanges(
		ctx,
		gr,
		fmt.Sprintf("[%s] bookmark added", gr.Name()),
	)
}

func Remove(ctx context.Context, app *application.App, bs []*bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	m, err := NewManager(app)
	if err != nil {
		return err
	}

	repoName := app.DBBaseName()
	if !m.IsTracked(repoName) {
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	gr := NewRepo(m, repoName, RepoStatsReader(r))
	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return m.SaveChanges(
		ctx,
		gr,
		fmt.Sprintf("[%s] remove bookmarks", repoName),
	)
}

func Drop(ctx context.Context, app *application.App, c *ui.Console) error {
	slog.Debug("git repo: start repo drop")
	if !app.GitEnabled() {
		slog.Debug("git repo: git disable")
		return nil
	}

	m, err := NewManager(app)
	if err != nil {
		return err
	}

	name := app.DBBaseName()
	if !m.IsTracked(name) || !files.Exists(app.Path.DB()) {
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	if !c.Confirm(ctx, "drop git repo?", "n") {
		return nil
	}

	gr := NewRepo(m, r.Name(), RepoStatsReader(r))
	if err := m.Drop(ctx, gr); err != nil {
		return err
	}

	if !c.Confirm(ctx, "untrack database?", "n") {
		return nil
	}

	if err := m.Untrack(ctx, gr, fmt.Sprintf("[%s] remove tracking", gr.Name())); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("database untracked\n"))
}

func Update(ctx context.Context, app *application.App, old, fresh *bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	m, err := NewManager(app)
	if err != nil {
		return err
	}

	if !m.IsEnabled() || !m.IsTracked(app.DBBaseName()) {
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	if err := r.UpdateOne(ctx, fresh); err != nil {
		return err
	}

	gr := NewRepo(m, r.Name(), RepoStatsReader(r))
	return m.Update(ctx, gr, old, fresh, files.RemoveEmptyDirs)
}
