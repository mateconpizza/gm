package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// MenuMainForRecords builds the interactive FZF menu for selecting records.
func MenuMainForRecords[T comparable](cfg *config.Config) *menu.Menu[T] {
	var keybindsArgs []string
	if cfg.Flags.Notes {
		keybindsArgs = append(keybindsArgs, "--notes")
	}

	mo := []menu.OptFn{
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(keybindsArgs...),
			config.FzfKeybindEditNotes(),
			config.FzfKeybindOpen(),
			config.FzfKeybindQR(),
			config.FzfKeybindOpenQR(),
			config.FzfKeybindYank(),
		),
	}

	if cfg.Flags.Multiline {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}

// MenuSimpleForRecords builds a simpler menu without all keybindings.
func MenuSimpleForRecords[T comparable](cfg *config.Config) *menu.Menu[T] {
	opts := []menu.OptFn{
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
	}

	if cfg.Flags.Multiline {
		opts = append(opts, menu.WithMultilineView())
	}

	return menu.New[T](opts...)
}

// MenuSimpleMultiRecords builds a simpler menu without all keybindings.
func MenuSimpleMultiRecords(cfg *config.Config) *menu.Menu[bookmark.Bookmark] {
	opts := []menu.OptFn{
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
		menu.WithMultiSelection(),
	}

	if cfg.Flags.Multiline {
		opts = append(opts, menu.WithMultilineView())
	}

	return menu.New[bookmark.Bookmark](opts...)
}

// selection allows the user to select a record in a menu interface.
func selection[T comparable](items []T, fmtFn func(*T) string, opts ...menu.OptFn) ([]T, error) {
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

// selectItem lets the user choose a repo from a list.
func selectItem(ctx context.Context, fs []string, header string) (string, error) {
	// FIX: inject `cfg`
	cfg := config.New()
	repos, err := selection(fs,
		func(p *string) string { return summary.RepoRecordsFromPath(ctx, *p) },
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(cfg.Cmd+" db -n {1} -i"),
	)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return repos[0], nil
}

// SelectBackupOne lets the user choose a backup and handles decryption if
// needed.
func SelectBackupOne(ctx context.Context, c *ui.Console, bks []string) (string, error) {
	// FIX: inject `cfg`
	cfg := config.New()
	selected, err := selection(bks,
		func(p *string) string { return summary.BackupWithFmtDateFromPath(ctx, *p) },
		menu.WithArgs("--cycle"),
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithPreview(cfg.Cmd+" db -n ./backup/{1} info"),
		menu.WithHeader("choose a backup to import from", false))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	backupPath := selected[0]

	// Handle locked backups
	if err := locker.IsLocked(backupPath); err != nil {
		if err := UnlockRepo(c, backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}

		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

func SelectBackupMany(ctx context.Context, root, header string) ([]string, error) {
	fs, err := files.FindByExtList(root, "db")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	// FIX: inject `cfg`
	cfg := config.New()

	repos, err := selection(fs,
		func(p *string) string { return summary.RepoRecordsFromPath(ctx, *p) },
		menu.WithMultiSelection(),
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithHeader(header, false),
		menu.WithPreview(cfg.Cmd+" db -n ./backup/{1} info"),
	)
	if err != nil {
		return repos, fmt.Errorf("%w", err)
	}

	return repos, nil
}

// SelectFileLocked lets the user choose a repo from a list of locked
// repos found in the given root directory.
func SelectFileLocked(ctx context.Context, root, header string) ([]string, error) {
	bks, err := files.FindByExtList(root, "enc")
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	cfg := config.New()
	selected, err := selection(bks,
		func(p *string) string { return summary.BackupWithFmtDateFromPath(ctx, *p) },
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithHeader(header, false),
	)
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectDatabase(ctx context.Context, ignoreDBPath string) (string, error) {
	// FIX: inject `cfg`
	cfg := config.New()

	// build list of candidate .db files
	dbFiles, err := files.FindByExtList(cfg.Path.Data, ".db")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	dbs := slice.New(dbFiles...)
	dbs = dbs.Filter(func(r string) bool {
		return r != ignoreDBPath
	})

	// ask the user which one to import from
	s, err := selectItem(ctx, *dbs.Items(), "choose a database to import from")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	if !files.Exists(s) {
		return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}

	return s, nil
}
