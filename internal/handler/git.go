package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
)

// NewGit returns a new git manager.
func NewGit(repoPath string) (*git.Manager, error) {
	gCmd := "git"

	gitCmd, err := sys.Which(gCmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, gCmd)
	}

	return git.New(repoPath, git.WithCmd(gitCmd)), nil
}

// GitCommit commits the bookmarks to the git repo.
func GitCommit(gm *git.Manager, actionMsg string) error {
	actionMsg = strings.ToLower(actionMsg)
	if !gm.IsInitialized() {
		slog.Debug("git export: git not initialized", "action", actionMsg)
		return git.ErrGitNotInitialized
	}

	if err := gm.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	gr := gm.Tracker.Current()
	if err := gm.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !gm.Tracker.Contains(gr) {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.DBName)
	}

	if !files.Exists(gr.Path) {
		slog.Debug("repo path does not exist", "path", gr.Path)
		return nil
	}

	r, err := db.New(gr.DBPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := r.AllPtr()
	if err != nil {
		return fmt.Errorf("load records: %w", err)
	}

	if err := port.GitWrite(gm, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return commitIfChanged(gm, actionMsg)
}

// GitDropRepo removes the repo from the git repo.
func GitDropRepo(gm *git.Manager, mesg string) error {
	mesg = strings.ToLower(mesg)
	gr := gm.Tracker.Current()
	slog.Debug("dropping repo", "dbPath", gr.DBPath)

	if !gm.IsInitialized() {
		return fmt.Errorf("dropping repo: %w: %q", git.ErrGitNotInitialized, gr.DBName)
	}

	if !files.Exists(gr.Path) {
		slog.Debug("repo does not exist", "path", gr.Path)
		return fmt.Errorf("%w: %q", git.ErrGitRepoNotFound, gr.Path)
	}

	if err := files.RemoveAll(gr.Path); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gm.AddAll(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gm.Commit(fmt.Sprintf("[%s] %s", gr.DBName, mesg)); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(gm *git.Manager, actionMsg string) error {
	err := gitRepoSummaryRepoStats(gm)
	if err != nil {
		return err
	}

	// Check if any changes
	changed, _ := gm.HasChanges()
	if !changed {
		return git.ErrGitNothingToCommit
	}

	if err := gm.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := gm.Status()
	if err != nil {
		status = ""
	}

	if status != "" {
		status = "(" + status + ")"
	}

	msg := fmt.Sprintf("[%s] %s %s", gm.Tracker.Current().DBName, actionMsg, status)
	if err := gm.Commit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func gitRepoSummaryRepoStats(gm *git.Manager) error {
	var (
		gr          = gm.Tracker.Current()
		summary     *git.SyncGitSummary
		summaryPath = filepath.Join(gr.Path, git.SummaryFileName)
	)

	if !files.Exists(summaryPath) {
		// Create new summary with only RepoStats
		summary = git.NewSummary()
		if err := GitRepoStats(gr.DBPath, summary); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		// Load existing summary
		summary = git.NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := GitRepoStats(gr.DBPath, summary); err != nil {
			return fmt.Errorf("updating repo stats: %w", err)
		}
	}

	// Save updated or new summary
	if err := files.JSONWrite(summaryPath, summary, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return nil
}

// GitSummary returns a new SyncGitSummary.
func GitSummary(gm *git.Manager, version string) (*git.SyncGitSummary, error) {
	r, err := db.New(gm.Tracker.Current().DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	branch, err := gm.Branch()
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}

	remote, err := gm.Remote()
	if err != nil {
		remote = ""
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	summary := &git.SyncGitSummary{
		GitBranch:          branch,
		GitRemote:          remote,
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "SHA-256",
		ClientInfo: &git.ClientInfo{
			Hostname:   hostname,
			Platform:   runtime.GOOS,
			Architect:  runtime.GOARCH,
			AppVersion: version,
		},
		RepoStats: &git.RepoStats{
			Name:      r.Cfg.Name,
			Bookmarks: db.CountMainRecords(r),
			Tags:      db.CountTagsRecords(r),
			Favorites: db.CountFavorites(r),
		},
	}

	summary.GenerateChecksum()

	return summary, nil
}

// GitRepoStats returns a new RepoStats.
func GitRepoStats(dbPath string, summary *git.SyncGitSummary) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	summary.RepoStats = &git.RepoStats{
		Name:      r.Cfg.Name,
		Bookmarks: db.CountMainRecords(r),
		Tags:      db.CountTagsRecords(r),
		Favorites: db.CountFavorites(r),
	}

	summary.GenerateChecksum()

	return nil
}

// GitTrackExportCommit tracks and exports a database.
func GitTrackExportCommit(c *ui.Console, gm *git.Manager, mesg string) error {
	if !gm.IsInitialized() {
		return git.ErrGitNotInitialized
	}

	gr := gm.Tracker.Current()
	if !gm.Tracker.Contains(gr) {
		if !c.Confirm(fmt.Sprintf("Track database %q?", gr.DBName), "n") {
			return nil
		}

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking database %q", gr.DBName)).String())
	}

	if err := port.GitExport(gm); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gm.Tracker.Track(gr).Save(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := GitCommit(gm, mesg); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.DBName)))

	return nil
}
