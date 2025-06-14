package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

// GitCommit commits the bookmarks to the git repo.
func GitCommit(dbPath, repoPath, actionMsg string) error {
	if !git.IsInitialized(repoPath) {
		slog.Debug("git export: git not initialized")
		return nil
	}

	if !files.Exists(repoPath) {
		return nil
	}

	r, err := db.New(dbPath)
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
		return GitDropRepo(dbPath, repoPath, "Dropped")
	}

	dbName := filepath.Base(dbPath)
	root := filepath.Join(repoPath, files.StripSuffixes(dbName))
	if err := port.GitWrite(repoPath, root, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return commitIfChanged(dbPath, repoPath, actionMsg)
}

// gitSummaryUpdate updates the summary file.
func gitSummaryUpdate(dbPath, repoPath, version string) error {
	dbName := filepath.Base(dbPath)
	summaryPath := filepath.Join(repoPath, files.StripSuffixes(dbName), git.SummaryFileName)

	newSum, err := gitSummary(dbPath, repoPath, version)
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

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(dbPath, repoPath, actionMsg string) error {
	dbName := filepath.Base(dbPath)
	err := gitSummaryRepoStats(dbPath, repoPath)
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

	msg := fmt.Sprintf("[%s] %s %s", dbName, actionMsg, status)
	if err := git.CommitChanges(repoPath, msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func gitSummaryRepoStats(dbPath, repoPath string) error {
	dbName := files.StripSuffixes(filepath.Base(dbPath))
	var summary *git.SyncGitSummary
	summaryPath := filepath.Join(repoPath, dbName, git.SummaryFileName)

	if !files.Exists(summaryPath) {
		// Create new summary with only RepoStats
		summary = git.NewSummary()
		if err := gitRepoStats(dbPath, summary); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		// Load existing summary
		summary = git.NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := gitRepoStats(dbPath, summary); err != nil {
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

// gitRepoStats returns a new RepoStats.
func gitRepoStats(dbPath string, summary *git.SyncGitSummary) error {
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

func GitSummaryGenerate(root, repoPath, version string) error {
	tracked, err := files.ListRootFolders(repoPath, ".git")
	if err != nil {
		return fmt.Errorf("listing tracked: %w", err)
	}

	// Generate the new summary
	for _, r := range tracked {
		dbPath := filepath.Join(root, files.EnsureSuffix(r, ".db"))
		if err := gitSummaryUpdate(dbPath, repoPath, version); err != nil {
			return fmt.Errorf("updating summary: %w", err)
		}
	}

	return nil
}

// gitCleanFiles removes the files from the git repo.
func gitCleanFiles(repoPath string, r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
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

	return GitCommit(r.Cfg.Fullpath(), repoPath, "Remove")
}

// GitTrackRemoveRepo removes a tracked repo from the git repository.
func GitTrackRemoveRepo(dbName, repoPath string, tracked []string) error {
	idx := slices.Index(tracked, files.StripSuffixes(dbName))
	if idx == -1 {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, dbName)
	}

	tracked = slices.Delete(tracked, idx, idx+1)
	if err := git.SetTracked(repoPath, tracked); err != nil {
		return fmt.Errorf("%w", err)
	}

	repoName := tracked[idx]
	repoToRemove := filepath.Join(repoPath, repoName)
	if err := files.RemoveAll(repoToRemove); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := git.AddAll(repoPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := git.CommitChanges(repoPath, fmt.Sprintf("[%s] Untrack database", dbName)); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.SuccessMesg(fmt.Sprintf("database %q untracked\n", dbName)))

	return nil
}

func GitTrackAddRepo(dbPath, repoPath string, tracked []string) error {
	dbName := filepath.Base(dbPath)
	if slices.Contains(tracked, files.StripSuffixes(dbName)) {
		return fmt.Errorf("%w: %q", git.ErrGitTracked, dbName)
	}

	tracked = append(tracked, dbPath)
	if err := git.SetTracked(repoPath, tracked); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := GitInitTracking(repoPath, tracked); err != nil {
		return err
	}
	fmt.Print(txt.SuccessMesg(fmt.Sprintf("database %q tracked\n", dbName)))

	return nil
}

// GitInitTracking initializes a tracked repo in the git repository.
func GitInitTracking(repoPath string, tracked []string) error {
	initializer := gitInitJSONRepo
	if gpg.IsInitialized(repoPath) {
		initializer = gitInitGPGRepo
	}

	for _, dbPath := range tracked {
		if err := initializer(dbPath, repoPath); err != nil {
			return err
		}
	}

	return nil
}

// gitInitGPGRepo creates a GPG repo for a tracked database.
func gitInitGPGRepo(dbPath, repoPath string) error {
	dbName := files.StripSuffixes(filepath.Base(dbPath))
	if files.Exists(filepath.Join(repoPath, dbName)) {
		return nil
	}

	if err := port.GitExport(dbPath); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			wmesg := fmt.Sprintf("Skipping %q, no bookmarks found\n", filepath.Base(dbPath))
			fmt.Print(txt.WarningMesg(wmesg))
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	if err := GitCommit(dbPath, repoPath, "Initializing encrypted repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.SuccessMesg("GPG repository initialized\n"))

	return nil
}

// gitInitJSONRepo creates a JSON repo for a tracked database.
func gitInitJSONRepo(dbPath, repoPath string) error {
	if err := port.GitExport(dbPath); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			wmesg := fmt.Sprintf("Skipping %q, no bookmarks found\n", filepath.Base(dbPath))
			fmt.Print(txt.WarningMesg(wmesg))
			return nil
		}
		return fmt.Errorf("%w", err)
	}

	if err := GitCommit(dbPath, repoPath, "Initializing repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.SuccessMesg("JSON repository initialized\n"))

	return nil
}

// GitTrackManagement updates the tracked databases in the git repository.
func GitTrackManagement(t *terminal.Term, f *frame.Frame, repoPath string) error {
	updatedTracked, err := SelectecTrackedDB(t, f, repoPath)
	if err != nil {
		return fmt.Errorf("select tracked: %w", err)
	}
	if err := git.SetTracked(repoPath, updatedTracked); err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := GitInitTracking(repoPath, updatedTracked); err != nil {
		return err
	}

	fmt.Print(txt.SuccessMesg("tracked databases updated\n"))

	return nil
}
