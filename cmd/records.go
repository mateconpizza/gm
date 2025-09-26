// Package cmd implements the command-line interface the bookmark manager.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var tagsFlags *config.Flags

func init() {
	initRecordFlags(recordsCmd)

	tagsFlags = config.NewFlags()

	recordsTagsCmd.Flags().
		BoolVarP(&config.App.Flags.JSON, "json", "j", false, "output tags+count in JSON format")
	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.List, "list", "l", false, "list all tags")

	recordsCmd.AddCommand(recordsTagsCmd)
	Root.AddCommand(recordsCmd)
}

var (
	// recordsCmd records management.
	// main command.
	recordsCmd = &cobra.Command{
		Use:               "rec",
		Aliases:           []string{"r", "records"},
		Short:             "Records management",
		PersistentPreRunE: RequireDatabase,
		RunE:              recordsCmdFunc,
	}

	// recordsTagsCmd tags management.
	recordsTagsCmd = &cobra.Command{
		Use:     "tags",
		Aliases: []string{"t"},
		Short:   "Tags management",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.App
			switch {
			case cfg.Flags.JSON:
				return printer.TagsJSON(cfg.DBPath)
			case tagsFlags.List:
				return printer.TagsList(cfg.DBPath)
			}

			return cmd.Usage()
		},
	}
)

// recordsCmdFunc is the main command and entrypoint.
func recordsCmdFunc(cmd *cobra.Command, args []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	cfg := config.App
	bs, err := handler.Data(menuForRecords(cfg), r, args, cfg.Flags)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	return runRecords(c, r, bs, cfg.Flags)
}

func initRecordFlags(cmd *cobra.Command) {
	flag := config.App.Flags
	f := cmd.Flags()
	f.SortFlags = false

	// Primary actions
	f.BoolVarP(&flag.Copy, "copy", "c", false, "copy bookmark URL to clipboard")
	f.BoolVarP(&flag.Edit, "edit", "e", false, "edit bookmark with preferred text editor")
	f.BoolVarP(&flag.Menu, "menu", "m", false, "interactive menu mode using fzf")
	f.BoolVarP(&flag.Notes, "notes", "N", false, "display bookmark notes")
	f.BoolVarP(&flag.Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&flag.QR, "qr", "q", false, "generate QR code for bookmark URL")
	f.BoolVarP(&flag.Remove, "remove", "r", false, "remove bookmark by query or ID")

	// Output format
	f.StringVarP(&flag.Field, "field", "f", "", "output specific field [id|url|title|tags|notes]")
	f.BoolVarP(&flag.JSON, "json", "j", false, "output results in JSON format")
	f.BoolVarP(&flag.Multiline, "multiline", "M", false, "output in multiline format (fzf compatible)")
	f.BoolVarP(&flag.Oneline, "oneline", "O", false, "output in single line format (fzf compatible)")

	// Filtering and pagination
	f.IntVarP(&flag.Head, "head", "H", 0, "show first N bookmarks")
	f.StringSliceVarP(&flag.Tags, "tag", "t", nil, "filter bookmarks by tag(s)")
	f.IntVarP(&flag.Tail, "tail", "T", 0, "show last N bookmarks")

	// Maintenance operations
	f.BoolVarP(&flag.Snapshot, "snapshot", "S", false, "fetch metadata from Wayback Machine")
	f.BoolVarP(&flag.Status, "status", "s", false, "check HTTP status of bookmark URLs")
	f.BoolVarP(&flag.Update, "update", "u", false, "update bookmark metadata")
}

func runRecords(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark, f *config.Flags) error {
	switch {
	case f.Status:
		return handler.CheckStatus(c, r, bs)
	case f.Snapshot:
		return handler.Snapshot(c, r, bs)
	case f.Remove:
		return handler.Remove(c, r, bs)
	case f.Export:
		return handler.Export(bs)
	case f.Edit:
		return handler.Edit(c, r, bs)
	case f.Update:
		return handler.Update(c, r, bs)
	case f.Copy:
		return handler.Copy(bs)
	case f.Open && !f.QR:
		return handler.Open(c, r, bs)
	}

	switch {
	case f.Field != "":
		return printer.ByField(bs, f.Field)
	case f.QR:
		return handler.QR(bs, f.Open)
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

// menuForRecords returns a FZF menu for showing records.
func menuForRecords[T bookmark.Bookmark](cfg *config.AppConfig) *menu.Menu[T] {
	var keybindsArgs []string
	if cfg.Flags.Notes {
		keybindsArgs = append(keybindsArgs, "--notes")
	}

	mo := []menu.OptFn{
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(keybindsArgs...),
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
