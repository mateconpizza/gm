package picker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNoItems = errors.New("no items")

// NewMainMenu builds the interactive FZF menu for selecting records.
func NewMainMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	if !app.Flags.Menu {
		return nil
	}

	fm := app.UI.MenuFmt

	p := fm.Menu.Placeholder
	kb := menu.NewBindBuilder(app.Cmd, app.DBName).
		WithPlaceholder(p)
	k := app.Menu.DefaultKeymaps

	fm.Menu.Opts = append(
		fm.Menu.Opts,
		menu.WithBorderLabel(" "+app.Name+" "),
		menu.WithConfig(app.Menu),
		menu.WithMultiSelection(),
		menu.WithOutputColor(app.Flags.Color),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), strings.ReplaceAll(p, "+", ""))),
		menu.WithPrompt(app.Menu.Prompt),
		menu.WithHeaderFirst(),
		menu.WithHeaderLabel(" keybinds "),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithKeybinds(
			kb.From(k.Edit).Execute("edit"),
			kb.From(k.EditNotes).Execute("notes edit"),
			kb.From(k.Open).Execute("open"),
			kb.From(k.QR).Execute("qr"),
			kb.From(k.OpenQR).Execute("qr open"),
			kb.From(k.Yank).Execute("yank"),
			kb.Builtin(k.ToggleAll, menu.ToggleAll),
			kb.Builtin(k.Preview, menu.TogglePreview),
		),
	)

	m := menu.New[bookmark.Bookmark](fm.Menu.Opts...)
	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(), b)
	})

	return m
}

func NewWithFormatter(app *application.App, opts ...menu.Option) *menu.Menu[bookmark.Bookmark] {
	fm := app.UI.MenuFmt
	fm.Menu.Opts = append(fm.Menu.Opts, opts...)

	m := New[bookmark.Bookmark](app, fm.Menu.Opts...)
	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(), b)
	})

	return m
}

// New builds a simpler menu without all keybindings.
func New[T comparable](app *application.App, opts ...menu.Option) *menu.Menu[T] {
	opts = append(
		opts,
		menu.WithBorderLabel(" "+app.Name+" "),
		menu.WithOutputColor(app.Flags.Color),
		menu.WithConfig(app.Menu),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithHeaderFirst(),
	)

	return menu.New[T](opts...)
}

func Select[T comparable](items []T, opts ...menu.Option) ([]T, error) {
	opts = append(
		opts,
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithHeaderFirst(),
	)

	m := menu.New[T](opts...)

	items, err := m.Select(items)
	if err != nil {
		return nil, err
	}

	return items, err
}

// BookmarkWithMenu applies menu selection to bookmarks.
func BookmarkWithMenu(m *menu.Menu[bookmark.Bookmark], bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
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
