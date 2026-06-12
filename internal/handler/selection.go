package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

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
func selectItem(ctx context.Context, d *deps.Deps, fs []string, header string) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}
	repos, err := selection(
		fs,
		func(p *string) string {
			return summary.RepoRecordsFromPath(ctx, d.Console(), *p)
		},
		menu.WithOutputColor(app.Flags.Color),
		menu.WithConfig(app.Menu),
		menu.WithHeader(header),
		menu.WithPreview(menu.PreviewCmd(app.Command(), "{1}", "db info")),
	)
	if err != nil {
		return "", err
	}

	return repos[0], nil
}

// SelectFileLocked lets the user choose a repo from a list of locked
// repos found in the given root directory.
func SelectFileLocked(ctx context.Context, d *deps.Deps, root, header string) ([]string, error) {
	bks, err := files.FindByExtList(root, "enc")
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	selected, err := selection(
		bks,
		func(p *string) string {
			return summary.BackupWithFmtDateFromPath(ctx, d.Console(), *p)
		},
		menu.WithOutputColor(app.Flags.Color),
		menu.WithConfig(app.Menu),
		menu.WithHeader(header),
	)
	if err != nil {
		return bks, fmt.Errorf("%w", err)
	}

	return selected, nil
}

func SelectDatabase(ctx context.Context, d *deps.Deps, ignoreDBPath string) (string, error) {
	dbs, err := ListDatabases(ctx, d, ignoreDBPath)
	if err != nil {
		return "", err
	}

	// ask the user which one to import from
	s, err := selectItem(ctx, d, dbs, "choose a database to import from")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	if !files.Exists(s) {
		return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}

	return s, nil
}

// MenuSelection applies menu selection to bookmarks.
func MenuSelection(m *menu.Menu[bookmark.Bookmark], bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	// Create copy for menu selection
	bsCopy := make([]bookmark.Bookmark, 0, len(bs))
	for _, b := range bs {
		bsCopy = append(bsCopy, *b)
	}

	defFormatter := func(b *bookmark.Bookmark) string { return formatter.OnelineFunc(ui.NewConsole(), b) }
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
func ListDatabases(ctx context.Context, d *deps.Deps, exclude string) ([]string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}
	// build list of candidate .db files
	dbFiles, err := files.FindByExtList(app.Path.Home(), ".db")
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
