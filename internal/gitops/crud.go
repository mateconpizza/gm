package gitops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

type BookmarkStore interface {
	Name() string
	BaseName() string
	Stats(ctx context.Context, dest any) error
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
}

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

func Add(ctx context.Context, app *application.App, r BookmarkStore, b *bookmark.Bookmark) error {
	// FIX: remove `app`
	if !app.GitEnabled() {
		return nil
	}

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	name := r.BaseName()
	if !gm.IsEnabled() || !gm.IsTracked(name) {
		return nil
	}

	gr := NewRepo(gm, r.Name(), git.WithRepoStore(r))
	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}

	return gm.SaveChanges(
		ctx,
		gr,
		fmt.Sprintf("[%s] bookmark added", gr.Name()),
	)
}

func Remove(ctx context.Context, app *application.App, bs []*bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	repoName := app.DBBaseName()
	if !gm.IsTracked(repoName) {
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	gr := NewRepo(gm, repoName, RepoStatsReader(r))
	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return gm.SaveChanges(
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

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	name := app.DBBaseName()
	if !gm.IsTracked(name) || !files.Exists(app.Path.DB()) {
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

	gr := NewRepo(gm, r.Name(), RepoStatsReader(r))
	if err := gm.Drop(ctx, gr); err != nil {
		return err
	}

	if !c.Confirm(ctx, "untrack database?", "n") {
		return nil
	}

	if err := gm.Untrack(ctx, gr, fmt.Sprintf("[%s] remove tracking", gr.Name())); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("database untracked\n"))
}

func Update(ctx context.Context, app *application.App, old, fresh *bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	if !gm.IsEnabled() || !gm.IsTracked(app.DBBaseName()) {
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

	gr := NewRepo(gm, r.Name(), RepoStatsReader(r))
	return gm.UpdateAndSave(ctx, gr, old, fresh, files.RemoveEmptyDirs)
}
