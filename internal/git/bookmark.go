package git

import (
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// UpdateBookmark updates a bookmark in Git version control.
// Only proceeds if Git repository is initialized and tracking the database.
func UpdateBookmark(cfg *config.Config, oldB, newB *bookmark.Bookmark) error {
	if !cfg.Git.Enabled {
		return nil
	}

	gr, err := NewRepo(cfg.DBPath)
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
func AddBookmark(cfg *config.Config, b *bookmark.Bookmark) error {
	gr, err := NewRepo(cfg.DBPath)
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

func RemoveBookmarks(cfg *config.Config, bs []*bookmark.Bookmark) error {
	if !cfg.Git.Enabled {
		return nil
	}

	gr, err := NewRepo(cfg.DBPath)
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
