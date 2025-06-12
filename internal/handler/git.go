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
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
)

// GitCommit commits the bookmarks to the git repo.
func GitCommit(actionMsg string) error {
	if !git.IsInitialized(config.App.Path.Git) {
		slog.Debug("git export: git not initialized")
		return nil
	}

	repoPath := config.App.Path.Git
	if !files.Exists(repoPath) {
		return nil
	}

	r, err := db.New(config.App.DBPath)
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
		slog.Debug("no bookmarks found", "repo", repoPath)
		return GitDropRepo(config.App.DBPath, config.App.Path.Git, "Dropped")
	}

	root := filepath.Join(repoPath, files.StripSuffixes(config.App.DBName))
	if err := port.GitWrite(repoPath, root, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return commitIfChanged(repoPath, actionMsg)
}

// GitSummaryUpdate updates the summary file.
func GitSummaryUpdate(dbPath, repoPath string) error {
	dbName := filepath.Base(dbPath)
	summaryPath := filepath.Join(repoPath, files.StripSuffixes(dbName), git.SummaryFileName)

	newSum, err := GitSummary(dbPath, repoPath)
	if err != nil {
		return fmt.Errorf("generating summary: %w", err)
	}
	if err := files.JSONWrite(summaryPath, newSum, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return nil
}

// GitDropRepo removes the repo from the git repo.
func GitDropRepo(dbPath, repoPath, mesg string) error {
	slog.Debug("dropping repo", "dbPath", dbPath)
	if !git.IsInitialized(repoPath) {
		return nil
	}

	dirPath := filepath.Join(repoPath, filepath.Base(files.StripSuffixes(dbPath)))
	if !files.Exists(dirPath) {
		slog.Debug("repo does not exist", "path", dirPath)
		return nil
	}
	if err := files.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := git.AddAll(repoPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	msg := fmt.Sprintf("[%s] %s", filepath.Base(dbPath), mesg)
	if err := git.CommitChanges(repoPath, msg); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// gitStoreBookmarksInRepo stores the bookmarks in the git repo.
// func gitStoreBookmarksInRepo(repoPath, root string, bookmarks []*bookmark.Bookmark) error {
// 	if gpg.IsInitialized(repoPath) {
// 		if err := bookmark.GitStoreAsGPG(repoPath, bookmarks); err != nil {
// 			return fmt.Errorf("store as GPG: %w", err)
// 		}
// 		return nil
// 	}
//
// 	if err := bookmark.ExportAsJSON(root, bookmarks); err != nil {
// 		return fmt.Errorf("export bookmarks: %w", err)
// 	}
// 	// if err := diffDeletedBookmarks(root, bookmarks); err != nil {
// 	// 	return fmt.Errorf("diff deleted: %w", err)
// 	// }
// 	return nil
// }

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(repoPath, actionMsg string) error {
	err := gitSummaryRepoStats(repoPath)
	if err != nil {
		return err
	}

	// Check if any changes
	changed, _ := git.HasChanges(repoPath)
	if !changed {
		return git.ErrGitNothingToCommit
	}

	if err := git.AddAll(repoPath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := git.Status(repoPath)
	if err != nil {
		status = ""
	}

	if status != "" {
		status = "(" + status + ")"
	}

	msg := fmt.Sprintf("[%s] %s %s", config.App.DBName, actionMsg, status)
	if err := git.CommitChanges(repoPath, msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func gitSummaryRepoStats(repoPath string) error {
	dbName := files.StripSuffixes(config.App.DBName)
	summaryPath := filepath.Join(repoPath, dbName, git.SummaryFileName)

	var summary *git.SyncGitSummary

	if !files.Exists(summaryPath) {
		// Create new summary with only RepoStats
		summary = git.NewSummary()
		if err := GitRepoStats(summary, repoPath); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		// Load existing summary
		summary = git.NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := GitRepoStats(summary, repoPath); err != nil {
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
func GitSummary(dbPath, repoPath string) (*git.SyncGitSummary, error) {
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
			AppVersion: config.App.Info.Version,
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
func GitRepoStats(summary *git.SyncGitSummary, repoPath string) error {
	r, err := db.New(config.App.DBPath)
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

func GitSummaryGenerate(repoPath string) error {
	tracked, err := files.ListRootFolders(repoPath, ".git")
	if err != nil {
		return fmt.Errorf("listing tracked: %w", err)
	}

	// Generate the new summary
	for _, r := range tracked {
		dbPath := filepath.Join(config.App.Path.Data, files.EnsureSuffix(r, ".db"))
		if err := GitSummaryUpdate(dbPath, repoPath); err != nil {
			return fmt.Errorf("updating summary: %w", err)
		}
	}

	return nil
}

// GitCleanFiles removes the files from the git repo.
func GitCleanFiles(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	repoPath := config.App.Path.Git
	if !git.IsInitialized(repoPath) {
		return nil
	}

	fileExt := port.FileExtJSON
	if gpg.IsInitialized(repoPath) {
		fileExt = gpg.Extension
	}

	var cleaner func(string, []*bookmark.Bookmark) error
	switch fileExt {
	case port.FileExtJSON:
		cleaner = port.GitCleanJSON
	case gpg.Extension:
		cleaner = port.GitCleanGPG
	}

	rootRepo := filepath.Join(repoPath, files.StripSuffixes(r.Cfg.Name))
	if err := cleaner(rootRepo, bs.ItemsPtr()); err != nil {
		return fmt.Errorf("cleaning repo: %w", err)
	}

	return GitCommit("Remove")
}
