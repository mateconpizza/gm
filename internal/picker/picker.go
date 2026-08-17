package picker

import (
	"errors"
	"fmt"
	"strings"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNoItems = errors.New("no items")

var (
	HeaderKeymapFmt = func(bind, sep, desc string) string {
		return fmt.Sprintf(
			"%s%s%s",
			ansi.BrightYellow.Sprint(bind),
			ansi.Gray.Sprint(sep),
			ansi.BrightBlue.Sprint(desc),
		)
	}

	HeaderSeparatorFmt = func(sep string) string {
		return ansi.Dim.Sprint(sep)
	}
)

// NewMainMenu builds the interactive FZF menu for selecting records.
func NewMainMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	if !app.Flags.Menu {
		return nil
	}

	fm := app.Formatter()
	opts := fm.Menu.Opts

	p := fm.Menu.Placeholder()
	kb := menucfg.NewBindBuilder().
		WithCommand(app.Command()).
		WithDBName(app.DBBaseName()).
		WithPlaceholder(p.Multi())

	builtinKeymaps := app.Menu.LoadKeymaps(kb)

	opts = append(
		opts,
		menu.WithMultiSelection(),
		menu.WithKeybinds(builtinKeymaps...),

		// preview window
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithPreviewCmd(PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
		menu.WithPreviewWindow(PreviewWindowArg(app.Menu.Preview)),

		// header
		menu.WithHeaderLabel(" keybinds "),
		menu.WithHeaderKeymaps(),
	)

	m := New[bookmark.Bookmark](app, opts...)

	m.SetFormatter(func(b bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(), &b)
	})

	return m
}

func NewWithFormatter(app *application.App, fm formatter.Formatter, opts ...menu.Option) *menu.Menu[bookmark.Bookmark] {
	opts = append(opts, fm.Menu.Opts...)
	m := New[bookmark.Bookmark](app, opts...)
	m.SetFormatter(func(b bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(), &b)
	})

	return m
}

// New builds a simpler menu without all keybindings.
func New[T comparable](app *application.App, opts ...menu.Option) *menu.Menu[T] {
	opts = append(
		opts,
		// appearance
		menu.WithAnsi(),
		menu.WithBorderLabel(" "+app.Name+" "),
		menu.WithOutputColor(app.Flags.Color),
		menu.WithLayout(menu.LayoutDefault),
		menu.WithHeaderSeparator(app.Menu.Header.Sep),

		// prompt & defaults
		menu.WithPrompt(app.Menu.Prompt),
		menu.WithDefaults(app.Menu.Defaults),
		menu.WithArgsCustom(app.Menu.Arguments...),
		menu.WithoutHeader(!app.Menu.Header.Enabled),

		// preview & info
		menu.WithPreviewBorder(menu.BorderRounded),
		menu.WithInfo(menu.InfoStyleInlineRight),

		// behavior
		menu.WithCycle(),
		menu.WithSync(),
		menu.WithNoScrollbar(),

		// header
		menu.WithHeaderFirst(),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithHeaderKeymapFmt(HeaderKeymapFmt),
		menu.WithHeaderSeparatorFmt(HeaderSeparatorFmt),
	)

	return menu.New[T](opts...)
}

func Select[T comparable](items []T, opts ...menu.Option) ([]T, error) {
	opts = append(
		opts,
		menu.WithHeaderFirst(),
		menu.WithHeaderBorder(menu.BorderRounded),
		menu.WithPreviewBorder(menu.BorderRounded),
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

	defFormatter := func(b bookmark.Bookmark) string {
		return formatter.Default().Render(ui.NewConsole(), &b)
	}
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
func selectionWithMenu[T comparable](m *menu.Menu[T], items []T, fmtFn func(T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, menu.ErrNoItems
	}

	m.SetFormatter(fmtFn)

	var result []T
	result, err := m.Select(items)
	if err != nil {
		if errors.Is(err, menu.ErrActionAborted) {
			return nil, sys.ErrActionAborted
		}

		return nil, fmt.Errorf("%w", err)
	}

	if len(result) == 0 {
		return nil, ErrNoItems
	}

	return result, nil
}

// PreviewCmd builds an fzf preview command.
func PreviewCmd(command, dbName string, args ...string) string {
	// FIX: Use `--color=always` for fzf previews.
	// This will `force` the preview window in FZF to `always` display colors, but if
	// color is disable, FZF will handle the color strip but keeps text styles
	// (dim, bold, italic, etc)
	return fmt.Sprintf(
		"%s --preview=frame --color=always --db=%s %s",
		command,
		dbName,
		strings.Join(args, " "),
	)
}

func PreviewWindowArg(show bool) string {
	if show {
		return "~4,+{2}+4/3,<80(up)"
	}
	return "hidden,up"
}
