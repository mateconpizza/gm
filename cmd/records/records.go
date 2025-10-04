// Package records provides Cobra subcommands for managing bookmarks and related
// entities, including record queries, actions, and tag operations.
package records

import (
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

var (
	// records is the root "records" command.
	// It provides entrypoints for listing, filtering, and operating on bookmarks.
	records = &cobra.Command{
		Use:     "rec",
		Aliases: []string{"r", "records"},
		Short:   "Records management",
		RunE:    CmdFunc,
	}

	// tagsCmd manages bookmark tags (list, JSON export, etc.).
	tagsCmd = &cobra.Command{
		Use:     "tags",
		Aliases: []string{"t"},
		Short:   "Tags management",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := config.New()
			switch {
			case app.Flags.JSON:
				return printer.TagsJSON(app.DBPath)
			case app.Flags.List:
				return printer.TagsList(app.DBPath)
			}

			return cmd.Usage()
		},
	}
)

// CmdFunc is the main command and entrypoint.
func CmdFunc(cmd *cobra.Command, args []string) error {
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

	c := ui.NewDefaultConsole(func(err error) {
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

	// Primary actions
	f.BoolVarP(&flag.Copy, "copy", "c", false,
		"copy bookmark URL to clipboard")
	f.BoolVarP(&flag.Edit, "edit", "e", false,
		"edit bookmark with preferred text editor")
	f.BoolVarP(&flag.Menu, "menu", "m", false,
		"interactive menu mode using fzf")
	f.BoolVarP(&flag.Notes, "notes", "N", false,
		"display bookmark notes")
	f.BoolVarP(&flag.Open, "open", "o", false,
		"open bookmark in default browser")
	f.BoolVarP(&flag.QR, "qr", "q", false,
		"generate QR code for bookmark URL")
	f.BoolVarP(&flag.Remove, "remove", "r", false,
		"remove bookmark by query or ID")

	// Output format
	f.StringVarP(&flag.Field, "field", "f", "",
		"output specific field [id|url|title|tags|notes]")
	f.BoolVarP(&flag.JSON, "json", "j", false,
		"output results in JSON format")
	f.BoolVarP(&flag.Multiline, "multiline", "M", false,
		"output in multiline format (fzf compatible)")
	f.BoolVarP(&flag.Oneline, "oneline", "O", false,
		"output in single line format (fzf compatible)")

	// Filtering and pagination
	f.IntVarP(&flag.Head, "head", "H", 0,
		"show first N bookmarks")
	f.StringSliceVarP(&flag.Tags, "tag", "t", nil,
		"filter bookmarks by tag(s)")
	f.IntVarP(&flag.Tail, "tail", "T", 0,
		"show last N bookmarks")

	// Maintenance operations
	f.BoolVarP(&flag.Snapshot, "snapshot", "S", false,
		"fetch metadata from Wayback Machine")
	f.BoolVarP(&flag.Status, "status", "s", false,
		"check HTTP status of bookmark URLs")
	f.BoolVarP(&flag.Update, "update", "u", false,
		"update bookmark metadata")
}

// exec handles the bookmark actions and output selection according to the
// provided flags.
func exec(c *ui.Console, r *db.SQLite, a *config.Config, bs []*bookmark.Bookmark) error {
	f := a.Flags
	switch {
	case f.Status:
		return handler.CheckStatus(c, r, bs)
	case f.Snapshot:
		return handler.Snapshot(c, r, bs)
	case f.Remove:
		return handler.Remove(c, r, bs, a)
	case f.Export:
		return handler.Export(bs)
	case f.Edit:
		return handler.Edit(c, r, a, bs)
	case f.Update:
		return handler.Update(c, r, a, bs)
	case f.Copy:
		return handler.Copy(bs)
	case f.Open && !f.QR:
		return handler.Open(c, r, bs)
	}

	switch {
	case f.Field != "":
		return printer.ByField(bs, f.Field)
	case f.QR:
		return handler.QR(bs, f.Open, a.Name)
	case f.JSON:
		return printer.RecordsJSON(bs)
	case f.Notes:
		return printer.Notes(bs)
	case f.Oneline:
		return printer.Oneline(bs)
	default:
		return printer.Records(bs)
	}
}

// menuForRecords builds the interactive FZF menu for selecting records.
func menuForRecords[T bookmark.Bookmark](cfg *config.Config) *menu.Menu[T] {
	var keybindsArgs []string
	if cfg.Flags.Notes {
		keybindsArgs = append(keybindsArgs, "--notes")
	}

	mo := []menu.OptFn{
		menu.WithSettings(config.Fzf.Settings),
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

// NewCmd creates and returns the top-level "records" Cobra command, including
// all subcommands and flags.
func NewCmd() *cobra.Command {
	app := config.New()
	InitFlags(records, app)

	tagsCmd.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false,
		"output tags+count in JSON format")
	tagsCmd.Flags().BoolVarP(&app.Flags.List, "list", "l", false,
		"list all tags")

	records.AddCommand(tagsCmd)

	return records
}
