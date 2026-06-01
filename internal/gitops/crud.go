package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

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
	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(app.Path.Git(), git.WithGit(g))
	if err != nil {
		return err
	}

	repoName := app.DBBaseName()
	if !m.IsTracked(repoName) {
		return nil
	}

	r, err := db.New(ctx, app.Path.Database)
	if err != nil {
		return err
	}
	defer r.Close()

	gr := m.NewRepo(
		repoName,
		RepoFileRemover(),
		RepoStatsReader(r),
	)
	if err := gr.RmMany(ctx, bs); err != nil {
		return err
	}

	return m.SaveChanges(ctx, git.NewCommitCfg(
		gr,
		app.Info.Version,
		fmt.Sprintf("[%s] remove bookmarks", repoName),
	))
}

func Drop(ctx context.Context, app *application.App) error {
	return nil
}

func Update(ctx context.Context, app *application.App, old, fresh *bookmark.Bookmark) error {
	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(
		app.Path.Git(),
		git.WithGit(g),
		MgrVersion(app.Info.Version),
	)
	if err != nil {
		return err
	}

	if !m.IsEnabled() || !m.IsTracked(app.DBBaseName()) {
		return nil
	}

	r, err := db.New(ctx, app.Path.Database)
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

	return m.Update(ctx, gr, old, fresh)
}

// Prune checks for differences between database and local repo and syncs them.
func Prune(ctx context.Context, app *application.App, r *db.SQLite) error {
	slog.Debug("git prune: starting repository prune")

	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(app.Path.Git(), git.WithGit(g))
	if err != nil {
		return err
	}
	if !m.IsEnabled() {
		slog.Debug("git prune: git not enabled")
		return nil
	}

	if !m.IsTracked(app.DBBaseName()) {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, app.DBBaseName())
	}

	inRepo, err := loadRepoBookmarks(ctx, app, m)
	if err != nil {
		return err
	}

	inDB, err := r.All(ctx)
	if err != nil {
		return err
	}

	added, err := syncMissingToRepo(ctx, app, inDB, inRepo)
	if err != nil {
		return fmt.Errorf("syncing missing files: %w", err)
	}

	removed, err := pruneStaleBookmarks(ctx, app, inRepo, inDB)
	if err != nil {
		return fmt.Errorf("pruning stale files: %w", err)
	}

	if added || removed {
		slog.Debug("git prune: state changed, updating summary")
		msg := fmt.Sprintf("[%s] sync repo", r.BaseName())
		if err := UpdateSummary(ctx, app, r, m, msg); err != nil {
			return fmt.Errorf("updating summary: %w", err)
		}
	}

	slog.Debug("git prune: repository already up to date")
	return git.ErrGitUpToDate
}

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

// UpdateSummary generates fresh stats from the DB and writes them to the Git repo.
func UpdateSummary(ctx context.Context, app *application.App, r *db.SQLite, m *git.Mgr, msg string) error {
	gr := m.NewRepo(
		app.DBBaseName(),
		RepoStatsReader(r),
	)

	return m.SaveChanges(ctx, git.NewCommitCfg(
		gr,
		app.Info.Version,
		msg,
	))
}
