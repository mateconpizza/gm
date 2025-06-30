package handler

import (
	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
)

// gitClean remove bookmarks files from git.
func gitClean(dbPath string, bs []*bookmark.Bookmark) error {
	repoPath := config.App.Git.Path
	if !git.IsInitialized(repoPath) {
		return nil
	}
	gr, err := git.NewRepo(dbPath)
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

// gitUpdate update bookmarks files in git.
func gitUpdate(dbPath string, oldB, newB *bookmark.Bookmark) error {
	repoPath := config.App.Git.Path
	if !git.IsInitialized(repoPath) {
		return nil
	}
	gr, err := git.NewRepo(dbPath)
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
