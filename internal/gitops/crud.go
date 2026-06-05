package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

func NewGit(app *application.App) (*git.Git, error) {
	return git.New(
		app.Path.Git(),
		git.WithGitCommandLogger(func(commands []string) {
			headerFrame := frame.New(frame.WithColorBorder(ansi.BrightYellow), frame.WithBordersSmallBlock())
			fullCmd := ansi.BrightYellow.Wrap(strings.Join(commands, " "), ansi.Italic)
			headerFrame.Midln(fullCmd).Flush()
		}),
	)
}

func Add(ctx context.Context, gitRoot string, r *db.SQLite, b *bookmark.Bookmark) error {
	m, err := git.NewManager(gitRoot)
	if err != nil {
		return err
	}

	repoName := r.BaseName()
	if !m.IsEnabled() || !m.IsTracked(repoName) {
		return nil
	}

	gr := m.NewRepo(repoName, RepoFileWriter())
	sum, err := getSummary(ctx, r, gr)
	if err != nil {
		return err
	}

	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}

	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	return m.Commit(ctx, fmt.Sprintf("[%s] bookmark added", gr.Name()))
}

func Remove(ctx context.Context, app *application.App, bs []*bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(
		app.Path.Git(),
		git.WithGit(g),
		git.WithVersion(app.Version()),
	)
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

	gr := m.NewRepo(
		repoName,
		RepoFileRemover(),
		RepoStatsReader(r),
	)

	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return m.SaveChanges(
		ctx,
		gr,
		fmt.Sprintf("[%s] remove bookmarks", repoName),
	)
}

func Drop(ctx context.Context, app *application.App) error {
	return nil
}

func Update(ctx context.Context, app *application.App, old, fresh *bookmark.Bookmark) error {
	if !app.GitEnabled() {
		return nil
	}

	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(
		app.Path.Git(),
		git.WithGit(g),
		MgrVersion(app.Version()),
	)
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

	gr := m.NewRepo(
		app.DBBaseName(),
		RepoFileRemover(),
		RepoFileWriter(),
		RepoStatsReader(r),
	)

	return m.Update(ctx, gr, old, fresh, files.RemoveEmptyDirs)
}
