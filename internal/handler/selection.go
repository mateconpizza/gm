package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
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
		menu.WithColor(cfg.Flags.Color),
		menu.WithConfig(cfg.Menu),
		menu.WithMultiSelection(),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
		menu.WithKeybinds(
			config.MenuKeybindEdit(keybindsArgs...),
			config.MenuKeybindEditNotes(),
			config.MenuKeybindOpen(),
			config.MenuKeybindQR(),
			config.MenuKeybindOpenQR(),
			config.MenuKeybindYank(),
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
		menu.WithColor(cfg.Flags.Color),
		menu.WithConfig(cfg.Menu),
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
		menu.WithColor(cfg.Flags.Color),
		menu.WithConfig(cfg.Menu),
		menu.WithMultiSelection(),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
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
func selectItem(a *app.Context, fs []string, header string) (string, error) {
	repos, err := selection(fs,
		func(p *string) string { return summary.RepoRecordsFromPath(a.Context(), a.Console(), *p) },
		menu.WithColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
		menu.WithPreview(a.Cfg.Cmd+" db -n {1} -i"),
	)
	if err != nil {
		return "", err
	}

	return repos[0], nil
}

// SelectBackupOne lets the user choose a backup and handles decryption if
// needed.
func SelectBackupOne(a *app.Context, bks []string) (string, error) {
	c := a.Console()
	selected, err := selection(bks,
		func(p *string) string { return summary.BackupWithFmtDateFromPath(a.Context(), c, *p) },
		menu.WithArgs("--cycle"),
		menu.WithColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader("choose a backup to import from"),
		menu.WithPreview(a.Cfg.Cmd+" db -n ./backup/{1} info"))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	backupPath := selected[0]

	// Handle locked backups
	if err := locker.IsLocked(backupPath); err != nil {
		if err := UnlockRepo(a, backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}

		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

func SelectBackupMany(a *app.Context, root, header string) ([]string, error) {
	fs, err := files.FindByExtList(root, "db")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	repos, err := selection(fs,
		func(p *string) string { return summary.RepoRecordsFromPath(a.Context(), a.Console(), *p) },
		menu.WithColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
		menu.WithMultiSelection(),
		menu.WithPreview(a.Cfg.Cmd+" db -n ./backup/{1} info"),
	)
	if err != nil {
		return repos, fmt.Errorf("%w", err)
	}

	return repos, nil
}

// SelectFileLocked lets the user choose a repo from a list of locked
// repos found in the given root directory.
func SelectFileLocked(a *app.Context, root, header string) ([]string, error) {
	bks, err := files.FindByExtList(root, "enc")
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	selected, err := selection(bks,
		func(p *string) string { return summary.BackupWithFmtDateFromPath(a.Context(), a.Console(), *p) },
		menu.WithColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
	)
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectDatabase(a *app.Context, ignoreDBPath string) (string, error) {
	// build list of candidate .db files
	dbFiles, err := files.FindByExtList(a.Cfg.Path.Data, ".db")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	dbs := slice.New(dbFiles...)
	dbs = dbs.Filter(func(r string) bool {
		return r != ignoreDBPath
	})

	// ask the user which one to import from
	s, err := selectItem(a, *dbs.Items(), "choose a database to import from")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	if !files.Exists(s) {
		return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}

	return s, nil
}
