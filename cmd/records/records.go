// Package records provides Cobra subcommands for managing bookmarks and related
// entities, including record queries, actions, and tag operations.
package records

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// NewCmd is the root "records" command.
// It provides entrypoints for listing, filtering, and operating on bookmarks.
func NewCmd() *cobra.Command {
	app := config.New()
	records := &cobra.Command{
		Use:     "rec",
		Aliases: []string{"r", "records"},
		Short:   "Records management",
		RunE:    Cmd,
	}

	InitFlags(records, app)

	return records
}

// Cmd is the main command and entrypoint.
func Cmd(cmd *cobra.Command, args []string) error {
	app := config.New()
	r, err := db.New(app.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(menuForRecords(app), r, args, app.Flags)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	return exec(c, r, app, bs)
}

// InitFlags initializes CLI flags for the records command.
func InitFlags(cmd *cobra.Command, cfg *config.Config) {
	flag := cfg.Flags
	f := cmd.Flags()
	f.SortFlags = false

	// Actions
	f.BoolVarP(&flag.Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&flag.Edit, "edit", "e", false, "edit bookmark with preferred text editor")
	f.BoolVarP(&flag.Remove, "remove", "r", false, "remove bookmark by query or ID")
	f.BoolVarP(&flag.Copy, "copy", "c", false, "copy bookmark URL to clipboard")
	f.BoolVarP(&flag.QR, "qr", "q", false, "generate QR code for bookmark URL")
	f.BoolVarP(&flag.Notes, "notes", "N", false, "display bookmark notes")
	f.BoolVarP(&flag.Menu, "menu", "m", false, "interactive menu mode using fzf")
	f.BoolVar(&flag.Multiline, "multiline", false, "output in multiline format (fzf)")

	// Display
	f.StringVarP(&cfg.Flags.Format, "format", "f", "",
		fmt.Sprintf("output format [%s]", strings.Join(printer.ValidFormats, "|")))

	// Filters
	InitFilterFlags(cmd, cfg)
}

func InitFilterFlags(cmd *cobra.Command, app *config.Config) {
	f := cmd.Flags()
	f.StringSliceVarP(&app.Flags.Tags, "tag", "t", nil, "filter bookmarks by tag(s)")
	f.IntVarP(&app.Flags.Head, "head", "H", 0, "show first N bookmarks")
	f.IntVarP(&app.Flags.Tail, "tail", "T", 0, "show last N bookmarks")
}

// exec handles the bookmark actions and output selection according to the
// provided flags.
func exec(c *ui.Console, r *db.SQLite, a *config.Config, bs []*bookmark.Bookmark) error {
	f := a.Flags
	switch {
	case f.Remove:
		return handler.Remove(c, r, bs, a)
	case f.Export:
		return handler.Export(bs)
	case f.Edit:
		return handler.Edit(c, r, a, bs)
	case f.Copy:
		return handler.Copy(bs)
	case f.Open && !f.QR:
		return handler.Open(c, r, bs)
	}

	switch {
	case f.Format != "":
		return printer.Display(f.Format, bs)
	case f.QR:
		return handler.QR(bs, f.Open, a.Name)
	case f.Notes:
		return printer.Notes(bs)
	default:
		return printer.Records(bs)
	}
}

// menuForRecords builds the interactive FZF menu for selecting records.
func menuForRecords[T bookmark.Bookmark](app *config.Config) *menu.Menu[T] {
	var keybindsArgs []string
	if app.Flags.Notes {
		keybindsArgs = append(keybindsArgs, "--notes")
	}

	mo := []menu.OptFn{
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(app.Cmd + " --name " + app.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(keybindsArgs...),
			config.FzfKeybindEditNotes(),
			config.FzfKeybindOpen(),
			config.FzfKeybindQR(),
			config.FzfKeybindOpenQR(),
			config.FzfKeybindYank(),
		),
	}

	if app.Flags.Multiline {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
