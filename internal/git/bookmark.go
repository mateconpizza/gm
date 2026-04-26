package git

import (
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// UpdateBookmark updates a bookmark in Git version control.
// Only proceeds if Git repository is initialized and tracking the database.
func UpdateBookmark(app *application.App, oldB, newB *bookmark.Bookmark) error {
	if !app.Git.Enabled {
		return nil
	}

	gr, err := NewRepo(app.Path.Database)
	if err != nil {
		return err
	}

	if !gr.IsTracked() {
		return nil
	}

	if err := gr.UpdateOne(oldB, newB); err != nil {
		return err
	}

	return gr.Commit("update bookmark")
}

// AddBookmark adds a bookmark to Git version control if the repository is tracked.
// Stages the bookmark, updates repository statistics, and creates a commit.
func AddBookmark(app *application.App, b *bookmark.Bookmark) error {
	gr, err := NewRepo(app.Path.Database)
	if err != nil {
		return err
	}

	if !gr.IsTracked() {
		return nil
	}

	if err := gr.Add([]*bookmark.Bookmark{b}); err != nil {
		return err
	}

	if err := gr.RepoStatsWrite(); err != nil {
		return err
	}

	return gr.Commit("new bookmark")
}

func RemoveBookmarks(app *application.App, bs []*bookmark.Bookmark) error {
	if !app.Git.Enabled {
		return nil
	}

	gr, err := NewRepo(app.Path.Database)
	if err != nil {
		return err
	}

	if !gr.IsTracked() {
		return nil
	}

	if err := gr.Remove(bs); err != nil {
		return err
	}

	return gr.Commit("remove bookmarks")
}
