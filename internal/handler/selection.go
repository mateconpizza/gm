package handler

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// Select allows the user to select a record in a menu interface.
func Select[T comparable](items []T, fmtFn func(*T) string, opts ...menu.OptFn) ([]T, error) {
	if len(items) == 0 {
		return nil, menu.ErrFzfNoItems
	}
	m := menu.New[T](opts...)
	selected, err := selectionWithMenu(m, items, fmtFn)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return selected, nil
}

// selectionWithMenu allows the user to select multiple records in a menu
// interface.
func selectionWithMenu[T comparable](m *menu.Menu[T], items []T, fmtFn func(*T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, menu.ErrFzfNoItems
	}

	m.SetPreprocessor(fmtFn)
	m.SetItems(items)

	var result []T
	result, err := m.Select()
	if err != nil {
		if errors.Is(err, menu.ErrFzfActionAborted) {
			return nil, sys.ErrActionAborted
		}

		return nil, fmt.Errorf("%w", err)
	}

	if len(result) == 0 {
		return nil, ErrNoItems
	}

	return result, nil
}

// SelectRepoBackup lets the user choose a backup and handles decryption if
// needed.
func SelectRepoBackup(destDB *db.SQLiteRepository) (string, error) {
	bks, err := destDB.ListBackups()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	selected, err := Select(bks,
		func(p *string) string { return db.BackupSummaryWithFmtDateFromPath(*p) },
		menu.WithArgs("--cycle"),
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		menu.WithHeader("choose a backup to import from", false))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	backupPath := selected[0]
	// Handle locked backups
	if err := locker.IsLocked(backupPath); err != nil {
		if err := UnlockRepo(terminal.New(), backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}
		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

// SelectItemFrom lets the user choose a repo from a list.
func SelectItemFrom(fs []string, header string) (string, error) {
	repos, err := Select(fs,
		func(p *string) string { return db.RepoSummaryRecordsFromPath(*p) },
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(config.App.Cmd+" db -n {1} info"),
	)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return repos[0], nil
}

// SelectRepo selects a repo.
func SelectRepo(args []string) (string, error) {
	var repoPath string
	fs, err := files.Find(config.App.Path.Data, "/*.db*")
	if err != nil {
		return repoPath, fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return repoPath, fmt.Errorf("%w", db.ErrDBsNotFound)
	}
	if len(args) == 0 {
		repoPath, err = SelectItemFrom(fs, "select database to remove")
		if err != nil {
			return repoPath, fmt.Errorf("%w", err)
		}
	} else {
		repoName := args[0]
		for _, r := range fs {
			repoName = files.EnsureSuffix(repoName, ".db")
			s := filepath.Base(r)
			if s == repoName || s == repoName+".enc" {
				repoPath = r
				break
			}
		}
	}
	if repoPath == "" {
		return repoPath, fmt.Errorf("%w: %q", db.ErrDBNotFound, args[0])
	}
	if !files.Exists(repoPath) {
		return repoPath, fmt.Errorf("%w: %q", db.ErrDBNotFound, repoPath)
	}

	return repoPath, nil
}

func SelectBackup(root, header string) ([]string, error) {
	fs, err := files.FindByExtList(root, "db")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}
	repos, err := Select(fs,
		func(p *string) string { return db.RepoSummaryRecordsFromPath(*p) },
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
	)
	if err != nil {
		return repos, fmt.Errorf("%w", err)
	}

	return repos, nil
}

// SelectFileLocked lets the user choose a repo from a list of locked
// repos found in the given root directory.
func SelectFileLocked(root, header string) ([]string, error) {
	bks, err := files.FindByExtList(root, "enc")
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}
	selected, err := Select(bks,
		func(p *string) string { return db.BackupSummaryWithFmtDateFromPath(*p) },
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
	)
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectFile(header string, search func() ([]string, error)) ([]string, error) {
	m := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
	)
	fs, err := search()
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}
	selected, err := selectionWithMenu(m, fs, func(p *string) string {
		return *p
	})
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectDatabase() (*db.SQLiteRepository, error) {
	// build list of candidate .db files
	dbFiles, err := files.FindByExtList(config.App.Path.Data, ".db")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	dbs := slice.New(dbFiles...)
	dbs = dbs.Filter(func(r string) bool {
		return filepath.Base(r) != config.App.DBName
	})
	// ask the user which one to import from
	s, err := SelectItemFrom(*dbs.Items(), "choose a database to import from")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if !files.Exists(s) {
		return nil, fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}
	// open source and destination
	srcDB, err := db.New(s)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return srcDB, nil
}

// SelectecTrackedDB prompts user to select which databases to track.
func SelectecTrackedDB(t *terminal.Term, f *frame.Frame, repoPath string) ([]string, error) {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return nil, fmt.Errorf("finding db files: %w", err)
	}

	if len(dbFiles) == 1 {
		dbName := files.StripSuffixes(filepath.Base(dbFiles[0]))
		f.Clear().Success(fmt.Sprintf("Tracking %q\n", dbName)).Flush()
		return dbFiles, nil
	}

	f.Ln().Midln("Select which databases to track").Flush()
	tracked := make([]string, 0, len(dbFiles))

	for _, dbFile := range dbFiles {
		f.Clear()
		dbName := files.StripSuffixes(filepath.Base(dbFile))
		if files.Exists(filepath.Join(repoPath, dbName)) {
			f.Info(filepath.Base(dbFile) + " is already tracked\n").Flush()
			continue
		}
		if !t.Confirm(f.Clear().Question(fmt.Sprintf("Track %q?", dbName)).String(), "n") {
			t.ClearLine(1)
			f.Clear().Info(fmt.Sprintf("Skipping %q\n", dbName)).Flush()
			continue
		}
		tracked = append(tracked, dbFile)
		t.ReplaceLine(1, f.Clear().Success(fmt.Sprintf("Tracking %q", dbName)).String())
	}

	if len(tracked) == 0 {
		return nil, terminal.ErrActionAborted
	}

	return tracked, nil
}
