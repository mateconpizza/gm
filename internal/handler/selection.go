package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// MenuMainForRecords builds the interactive FZF menu for selecting records.
func MenuMainForRecords[T comparable](cfg *config.Config) *menu.Menu[T] {
	kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
	k := cfg.Menu.DefaultKeymaps
	mo := []menu.Option{
		menu.WithBorderLabel(" " + cfg.Name + " "),
		menu.WithConfig(cfg.Menu),
		menu.WithMultiSelection(),
		menu.WithOutputColor(cfg.Flags.Color),
		menu.WithPreview(cfg.PreviewCmd(cfg.DBName) + " {1}"),
		menu.WithPrompt(cfg.Menu.Prompt),
		menu.WithHeaderFirst(),
		menu.WithHeaderLabel(" keybinds "),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithNth("3", "4"),
		menu.WithKeybinds(
			kb.Edit(k.Edit),
			kb.EditNotes(k.EditNotes),
			kb.Open(k.Open),
			kb.QR(k.QR),
			kb.QROpen(k.OpenQR),
			kb.Yank(k.Yank),
			kb.ToggleAll(k.ToggleAll),
			kb.Preview(k.Preview),
		),
	}

	return menu.New[T](mo...)
}

// MenuSimple builds a simpler menu without all keybindings.
func MenuSimple[T comparable](cfg *config.Config, opts ...menu.Option) *menu.Menu[T] {
	opts = append(opts,
		menu.WithBorderLabel(" "+cfg.Name+" "),
		menu.WithOutputColor(cfg.Flags.Color),
		menu.WithConfig(cfg.Menu),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithHeaderFirst(),
	)

	return menu.New[T](opts...)
}

// selection allows the user to select a record in a menu interface.
func selection[T comparable](items []T, fmtFn func(*T) string, opts ...menu.Option) ([]T, error) {
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

	m.SetFormatter(fmtFn)

	var result []T
	result, err := m.Select(items)
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
		menu.WithOutputColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
		menu.WithPreview(a.Cfg.PreviewCmd("{1}")+" db info"),
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
		menu.WithOutputColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader("choose a backup to import from"),
		menu.WithPreview(a.Cfg.PreviewCmd("./backup/{1}")+" db info"),
	)
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
		menu.WithOutputColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
		menu.WithMultiSelection(),
		menu.WithPreview(a.Cfg.PreviewCmd("./backup/{1}", "db info")),
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
		menu.WithOutputColor(a.Cfg.Flags.Color),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithHeader(header),
	)
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectDatabase(a *app.Context, ignoreDBPath string) (string, error) {
	dbs, err := ListDatabases(a, ignoreDBPath)
	if err != nil {
		return "", err
	}

	// ask the user which one to import from
	s, err := selectItem(a, dbs, "choose a database to import from")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	if !files.Exists(s) {
		return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}

	return s, nil
}

// ApplyMenuSelection applies menu selection to bookmarks.
func ApplyMenuSelection(
	c *ui.Console,
	m *menu.Menu[bookmark.Bookmark],
	bs []*bookmark.Bookmark,
) ([]*bookmark.Bookmark, error) {
	// Create copy for menu selection
	bsCopy := make([]bookmark.Bookmark, 0, len(bs))
	for _, b := range bs {
		bsCopy = append(bsCopy, *b)
	}

	defFormatter := func(b *bookmark.Bookmark) string { return txt.Oneline(c, b) }
	if m.Formatter == nil {
		m.SetFormatter(defFormatter)
	}

	// Select with menu
	items, err := selectionWithMenu(m, bsCopy, m.Formatter)
	if err != nil {
		return nil, err
	}

	// Convert selected items back to pointers
	result := make([]*bookmark.Bookmark, len(items))
	for i := range items {
		result[i] = &items[i]
	}

	return result, nil
}

// ListDatabases returns database paths, excluding the given path.
func ListDatabases(a *app.Context, exclude string) ([]string, error) {
	// build list of candidate .db files
	dbFiles, err := files.FindByExtList(a.Cfg.Path.Data, ".db")
	if err != nil {
		return nil, err
	}

	dbs := make([]string, 0, len(dbFiles))
	for i := range dbFiles {
		if dbFiles[i] == exclude {
			continue
		}
		dbs = append(dbs, dbFiles[i])
	}

	return dbs, nil
}
