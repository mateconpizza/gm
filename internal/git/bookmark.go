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

	m, err := NewManager(app.Path.Database)
	if err != nil {
		return err
	}

	if !m.IsTracked() {
		return nil
	}

	if err := m.UpdateOne(oldB, newB); err != nil {
		return err
	}

	return m.Commit("update bookmark")
}

// AddBookmark adds a bookmark to Git version control if the repository is tracked.
// Stages the bookmark, updates repository statistics, and creates a commit.
func AddBookmark(dbPath string, b *bookmark.Bookmark) error {
	m, err := NewManager(dbPath)
	if err != nil {
		return err
	}

	if !m.IsTracked() {
		return nil
	}

	if err := m.Add([]*bookmark.Bookmark{b}); err != nil {
		return err
	}

	if err := m.WriteStats(); err != nil {
		return err
	}

	return m.Commit("bookmark added")
}

func RemoveBookmarks(app *application.App, bs []*bookmark.Bookmark) error {
	if !app.Git.Enabled {
		return nil
	}

	m, err := NewManager(app.Path.Database)
	if err != nil {
		return err
	}

	if !m.IsTracked() {
		return nil
	}

	if err := m.Remove(bs); err != nil {
		return err
	}

	return m.Commit("remove bookmarks")
}
