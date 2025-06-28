package handler

import (
	"fmt"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
)

// GitTrackExportCommit tracks and exports a database.
func GitTrackExportCommit(c *ui.Console, gr *git.Repository, mesg string) error {
	if gr.IsTracked() {
		return fmt.Errorf("%w: %q", git.ErrGitTracked, gr.Loc.DBName)
	}

	if !c.Confirm(fmt.Sprintf("Track database %q?", gr.Loc.DBName), "n") {
		c.ReplaceLine(c.Warning(fmt.Sprintf("Skipping database %q", gr.Loc.DBName)).String())

		return nil
	}
	c.ReplaceLine(c.Success(fmt.Sprintf("Tracking database %q", gr.Loc.DBName)).String())

	if err := gr.TrackAndCommit(); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Loc.DBName)))

	return nil
}

// GitUntrackDropCommit untracks and remove git repo.
func GitUntrackDropCommit(c *ui.Console, gr *git.Repository) error {
	if !gr.IsTracked() {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.Loc.DBName)
	}

	q := color.Text(fmt.Sprintf("Untrack %q?", gr.Loc.Name)).Bold()
	if !c.T.Confirm(c.Warning(q.String()).String(), "n") {
		c.ReplaceLine(c.Info(fmt.Sprintf("Unchange database %q", gr.Loc.Name)).String())

		return nil
	}

	c.ReplaceLine(c.Warning(fmt.Sprintf("Untracking database %q", gr.Loc.Name)).String())

	if err := gr.Untrack(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gr.Drop(fmt.Sprintf("[%s] %s", gr.Loc.DBName, "untrack database")); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.Loc.DBName)))

	return nil
}

// gitClean remove bookmarks files from git.
func gitClean(dbPath string, bs []*bookmark.Bookmark) error {
	repoPath := config.App.Path.Git
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
	return gr.Remove(bs)
}

// gitUpdate update bookmarks files in git.
func gitUpdate(dbPath string, oldB, newB *bookmark.Bookmark) error {
	repoPath := config.App.Path.Git
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
	if err := gr.Remove([]*bookmark.Bookmark{oldB}); err != nil {
		return err
	}
	return gr.Add([]*bookmark.Bookmark{newB})
}
