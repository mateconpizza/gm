package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/encryptor"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Selection allows the user to select multiple records in a menu
// interface.
func Selection[T comparable](m *menu.Menu[T], items []T, fmtFn func(*T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, repo.ErrRecordNoMatch
	}

	var result []T
	result, err := m.Select(items, fmtFn)
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

// SelectBackupFrom lets the user choose a backup and handles decryption if
// needed.
func SelectBackupFrom(destDB *repo.SQLiteRepository) (string, error) {
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
		if err := UnlockDB(terminal.New(), backupPath); err != nil {
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
