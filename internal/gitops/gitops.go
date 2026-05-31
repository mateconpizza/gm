package gitops

import (
	"context"
	"log/slog"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

// loadRepoBookmarks reads the git repo and returns the current bookmarks found there.
func loadRepoBookmarks(ctx context.Context, app *application.App, m *git.Mgr) ([]*bookmark.Bookmark, error) {
	gr := m.NewRepo(app.DBNameBase(), RepoFileReader())
	total, err := gr.Count()
	if err != nil {
		return nil, err
	}

	err = gr.Read(ctx, total)
	return gr.Bookmarks(), err
}

// syncMissingToRepo writes bookmarks that exist in the DB but are absent from the repo.
// Returns a boolean indicating if any files were actually added.
func syncMissingToRepo(ctx context.Context, app *application.App, inDB, inRepo []*bookmark.Bookmark) (bool, error) {
	// find bookmarks in DB that are NOT in the Repo
	missing, _ := port.Deduplicate(inDB, inRepo)

	if len(missing) == 0 {
		return false, nil
	}

	slog.Debug("git sync: found missing bookmarks", "count", len(missing))
	m, err := git.NewManager(app.Path.Git())
	if err != nil {
		return false, err
	}

	gr := m.NewRepo(app.DBNameBase(), RepoFileWriter())
	if err := gr.Add(ctx, missing); err != nil {
		return false, err
	}

	return true, nil
}

// pruneStaleBookmarks removes bookmarks that exist in the repo but are no longer in the DB.
func pruneStaleBookmarks(ctx context.Context, app *application.App, inRepo, inDB []*bookmark.Bookmark) (bool, error) {
	// Find bookmarks in Repo that are NOT in the DB
	stale, _ := port.Deduplicate(inRepo, inDB)

	if len(stale) == 0 {
		slog.Debug("git sync: no stale bookmarks found")
		return false, nil
	}

	m, err := git.NewManager(app.Path.Git())
	if err != nil {
		return false, err
	}

	gr := m.NewRepo(app.DBNameBase(), RepoFileRemover())
	if err := gr.RmMany(ctx, stale); err != nil {
		return false, err
	}

	return true, nil
}

func getSummary(ctx context.Context, r *db.SQLite, repo *git.Repo) (*git.Summary, error) {
	stats := git.NewRepoStats()
	if err := r.Stats(ctx, &stats); err != nil {
		return nil, err
	}

	sum, err := repo.Summary()
	if err != nil {
		return nil, err
	}

	stats.Name = repo.Name()
	sum.RepoStats = stats

	return sum, nil
}
