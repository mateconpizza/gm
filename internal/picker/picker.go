package picker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNoItems = errors.New("no items")

// New builds a simpler menu without all keybindings.
func New[T comparable](app *application.App, opts ...menu.Option) *menu.Menu[T] {
	all := append(themeOpts(app), opts...)
	return menu.New[T](all...)
}

// NewMainMenu builds the interactive FZF menu for selecting records.
func NewMainMenu(app *application.App) *menu.Menu[*bookmark.Bookmark] {
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

	m := New[*bookmark.Bookmark](app, opts...)
	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(
			ui.WithColor(app.Flags.Color),
			ui.WithGlyphs(app.Glyphs()),
		), b)
	})

	return m
}

func NewWithFormatter(app *application.App, fm formatter.Formatter, opts ...menu.Option) *menu.Menu[*bookmark.Bookmark] {
	opts = append(opts, fm.Menu.Opts...)
	m := New[*bookmark.Bookmark](app, opts...)

	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(ui.NewConsole(
			ui.WithColor(app.Flags.Color),
			ui.WithGlyphs(app.Glyphs()),
		), b)
	})

	return m
}

func Select[T comparable](ctx context.Context, items []T, opts ...menu.Option) ([]T, error) {
	m := menu.New[T](opts...)
	items, err := m.Select(ctx, items)
	if err != nil {
		return nil, err
	}

	return items, err
}

// BookmarkWithMenu applies menu selection to bookmarks.
func BookmarkWithMenu(ctx context.Context, c *ui.Console, m *menu.Menu[*bookmark.Bookmark], bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	if m.Formatter == nil {
		m.SetFormatter(func(b *bookmark.Bookmark) string {
			return formatter.Default().Render(c, b)
		})
	}
	return selectionWithMenu(ctx, m, bs)
}

// selectionWithMenu allows the user to select multiple records in a menu
// interface.
func selectionWithMenu[T comparable](ctx context.Context, m *menu.Menu[T], items []T) ([]T, error) {
	if len(items) == 0 {
		return nil, menu.ErrNoItems
	}

	result, err := m.Select(ctx, items)
	if err != nil {
		if errors.Is(err, menu.ErrActionAborted) {
			return nil, application.ErrActionAborted
		}

		return nil, fmt.Errorf("menu: %w", err)
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

// themeOpts returns the appearance/behavior options for the given theme.
func themeOpts(app *application.App) []menu.Option {
	switch app.Menu.Theme {
	case menucfg.ThemeMinimal:
		return minimalThemeOpts(app)
	default:
		return defaultThemeOpts(app)
	}
}

func commonThemeOpts(app *application.App) []menu.Option {
	return []menu.Option{
		menu.WithAnsi(),
		menu.WithOutputColor(app.Flags.Color),
		menu.WithLayout(menu.LayoutDefault),

		menu.WithPrompt(app.Menu.Prompt),
		menu.WithDefaults(app.Menu.Defaults),
		menu.WithArgsCustom(app.Menu.Arguments...),
		menu.WithInfo(menu.InfoStyleInlineRight),

		menu.WithNoScrollbar(),
		menu.WithSync(),
		menu.WithCycle(),

		menu.WithArgs(func(b *menu.ArgsBuilder) *menu.ArgsBuilder {
			return b.Custom("--no-separator", "--ghost=search")
		}),
	}
}

// defaultThemeOpts is the full, bordered, decorated theme.
func defaultThemeOpts(app *application.App) []menu.Option {
	p := ansi.NewPalette(app.Flags.Color)
	opts := commonThemeOpts(app)

	opts = append(opts,
		menu.WithBorderLabel(" "+app.Name+" "),
		menu.WithHeaderSeparator(app.Menu.Header.Sep),
		menu.WithoutHeader(!app.Menu.Header.Enabled),

		menu.WithPreviewBorder(menu.BorderRounded),

		menu.WithHeaderFirst(),
		menu.WithHeaderBorder(menu.BorderRounded),

		menu.WithHeaderKeymapFmt(func(bind, sep, desc string) string {
			return fmt.Sprintf(
				"%s%s%s",
				p.BrightYellow.Sprint(bind),
				p.Gray.Sprint(sep),
				p.BrightBlue.Sprint(desc),
			)
		}),

		menu.WithHeaderSeparatorFmt(func(sep string) string {
			return p.Dim.Sprint(sep)
		}),
	)

	return opts
}

// minimalThemeOpts strips visual decorations.
func minimalThemeOpts(app *application.App) []menu.Option {
	opts := commonThemeOpts(app)
	return append(opts, menu.WithoutHeader(true))
}
