package handler

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/encryptor"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Selection allows the user to select multiple records in a menu
// interface.
func Selection[T comparable](m *menu.Menu[T], items []T, fmtFn func(*T) string) ([]T, error) {
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
func SelectRepoBackup(destDB *repo.SQLiteRepository) (string, error) {
	mBks := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		menu.WithHeader("choose a backup to import from", false),
	)

	bks, err := destDB.BackupsList()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	selected, err := Selection(mBks, bks, func(p *string) string {
		return repo.BackupSummaryWithFmtDateFromPath(*p)
	})
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	backupPath := selected[0]

	// Handle encrypted backups
	if err := encryptor.IsEncrypted(backupPath); err != nil {
		if err := UnlockRepo(terminal.New(), backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}
		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

// SelectItemFrom lets the user choose a repo from a list.
func SelectItemFrom(fs []string, header string) (string, error) {
	m := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(config.App.Cmd+" db -n {1} info"),
	)
	repos, err := Selection(m, fs, func(p *string) string {
		return repo.RepoSummaryRecordsFromPath(*p)
	})
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return repos[0], nil
}

// SelectRepo selects a repo.
func SelectRepo(args []string) (string, error) {
	var repoPath string
	fs, err := filepath.Glob(config.App.Path.Data + "/*.db*")
	if err != nil {
		return repoPath, fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return repoPath, fmt.Errorf("%w", repo.ErrDBsNotFound)
	}
	if len(args) == 0 {
		repoPath, err = SelectItemFrom(fs, "select database to remove")
		if err != nil {
			return repoPath, fmt.Errorf("%w", err)
		}
	} else {
		repoName := args[0]
		for _, r := range fs {
			repoName = files.EnsureExt(repoName, ".db")
			s := filepath.Base(r)
			if s == repoName || s == repoName+".enc" {
				repoPath = r
				break
			}
		}
	}
	if repoPath == "" {
		return repoPath, fmt.Errorf("%w: %q", repo.ErrDBNotFound, args[0])
	}
	if !files.Exists(repoPath) {
		return repoPath, fmt.Errorf("%w: %q", repo.ErrDBNotFound, repoPath)
	}
	if err := encryptor.IsEncrypted(repoPath); err != nil {
		return repoPath, fmt.Errorf("%w", err)
	}

	return repoPath, nil
}

func SelectBackup(root, header string) ([]string, error) {
	fs, err := files.FindByExtList(root, "db")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}
	m := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
	)
	repos, err := Selection(m, fs, func(p *string) string {
		return repo.RepoSummaryRecordsFromPath(*p)
	})
	if err != nil {
		return repos, fmt.Errorf("%w", err)
	}

	return repos, nil
}

// SelectFileEncrypted lets the user choose a repo from a list of encrypted
// repos found in the given root directory.
func SelectFileEncrypted(root, header string) ([]string, error) {
	m := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(header, false),
	)
	bks, err := files.FindByExtList(root, "enc")
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}
	selected, err := Selection(m, bks, func(p *string) string {
		return repo.BackupSummaryWithFmtDateFromPath(*p)
	})
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
	selected, err := Selection(m, fs, func(p *string) string {
		return *p
	})
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	return selected, nil
}

// SelectSource lets the user choose a source to import from.
func SelectSource(cmd *cobra.Command, args []string) error {
	sources := map[string]func(*cobra.Command, []string) error{
		"backups":  ImportFromBackup,
		"browser":  ImportFromBrowser,
		"database": ImportFromDatabase,
	}

	menuFlag, err := cmd.Flags().GetBool("menu")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if !menuFlag {
		err := cmd.Help()
		return fmt.Errorf("%w", err)
	}
	d := format.NBSP
	s := []string{
		format.PaddedLine(color.BrightYellow("backups"), d+"Import bookmarks from backups"),
		format.PaddedLine(color.BrightGreen("browser"), d+"Import bookmarks from browser"),
		format.PaddedLine(color.BrightBlue("database"), d+"Import bookmarks from database"),
	}
	m := menu.New[string](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader("select a source to import from", false),
	)
	r, err := Selection(m, s, nil)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	c := color.ANSICodeRemover(strings.Split(r[0], d)[0])

	return sources[strings.TrimSpace(c)](cmd, args)
}
