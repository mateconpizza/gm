package handler

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
)

// newGit returns a new git manager.
func newGit(repoPath string) (*git.Manager, error) {
	gCmd := "git"
	gitCmd, err := sys.Which(gCmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, gCmd)
	}

	return git.New(repoPath, git.WithCmd(gitCmd)), nil
}

// GitCommit commits the bookmarks to the git repo.
func GitCommit(g *git.Manager, actionMsg string) error {
	if !g.IsInitialized() {
		slog.Debug("git export: git not initialized")
		return git.ErrGitNotInitialized
	}
	if err := g.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	gr := g.Tracker.Current()

	if !g.Tracker.IsTracked(gr) {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.DBName)
	}

	if !files.Exists(gr.Path) {
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

	// remove repo if no bookmarks
	if len(bookmarks) == 0 {
		slog.Debug("no bookmarks found", "database", gr.DBName)
		return GitDropRepo(g, "Dropped")
	}

	if err := port.GitWrite(g, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return commitIfChanged(g, actionMsg)
}

// GitDropRepo removes the repo from the git repo.
func GitDropRepo(g *git.Manager, mesg string) error {
	gr := g.Tracker.Current()
	slog.Debug("dropping repo", "dbPath", gr.DBPath)
	if !g.IsInitialized() {
		return fmt.Errorf("dropping repo: %w: %q", git.ErrGitNotInitialized, gr.DBName)
	}

	if !files.Exists(gr.Path) {
		slog.Debug("repo does not exist", "path", gr.Path)
		return fmt.Errorf("%w: %q", git.ErrGitRepoNotFound, gr.Path)
	}

	if err := files.RemoveAll(gr.Path); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := g.AddAll(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := g.Commit(fmt.Sprintf("[%s] %s", gr.DBName, mesg)); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(g *git.Manager, actionMsg string) error {
	err := gitRepoSummaryRepoStats(g)
	if err != nil {
		return err
	}

	// Check if any changes
	changed, _ := g.HasChanges()
	if !changed {
		return git.ErrGitNothingToCommit
	}

	if err := g.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := g.Status()
	if err != nil {
		status = ""
	}

	if status != "" {
		status = "(" + status + ")"
	}

	msg := fmt.Sprintf("[%s] %s %s", g.Tracker.Current().DBName, actionMsg, status)
	if err := g.Commit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func gitRepoSummaryRepoStats(g *git.Manager) error {
	gr := g.Tracker.Current()
	var summary *git.SyncGitSummary
	summaryPath := filepath.Join(gr.Path, gr.DBName, git.SummaryFileName)

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

// gitSummary returns a new SyncGitSummary.
func gitSummary(dbPath, repoPath, version string) (*git.SyncGitSummary, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}
	branch, err := git.GetBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}
	remote, err := git.GetRemote(repoPath)
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

// gitCleanFiles removes the files from the git repo.
func gitCleanFiles(g *git.Manager, bs *slice.Slice[bookmark.Bookmark]) error {
	if !g.IsInitialized() {
		return nil
	}

	fileExt := port.JSONFileExt
	if gpg.IsInitialized(g.RepoPath) {
		fileExt = gpg.Extension
	}

	var cleaner func(string, []*bookmark.Bookmark) error
	switch fileExt {
	case port.JSONFileExt:
		cleaner = port.GitCleanJSON
	case gpg.Extension:
		cleaner = port.GitCleanGPG
	}

	if err := cleaner(g.Tracker.Current().Path, bs.ItemsPtr()); err != nil {
		return fmt.Errorf("cleaning repo: %w", err)
	}

	return GitCommit(g, "Remove")
}
