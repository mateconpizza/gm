package cmd

import (
	"fmt"

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

func init() {
	initRecordFlags(recordsCmd)

	recordsTagsCmd.Flags().
		BoolVarP(&config.App.Flags.JSON, "json", "j", false, "output tags+count in JSON format")
	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.list, "list", "l", false, "list all tags")

	recordsCmd.AddCommand(recordsTagsCmd)
	Root.AddCommand(recordsCmd)
}

type tagsFlagType struct {
	list bool
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

	// tags flags.
	tagsFlags = tagsFlagType{}

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
			case tagsFlags.list:
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
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(menuForRecords(), r, args)
	if err != nil {
		return fmt.Errorf("%w", err)
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

	f := config.App.Flags

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

func initRecordFlags(cmd *cobra.Command) {
	cfg := config.App
	f := cmd.Flags()

	// Prints
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&cfg.Flags.Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	f.BoolVarP(&cfg.Flags.Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	f.StringVarP(&cfg.Flags.Field, "field", "f", "", "output by field [id|url|title|tags]")
	f.BoolVarP(&cfg.Flags.Notes, "notes", "N", false, "notes")

	// Actions
	f.BoolVarP(&cfg.Flags.Copy, "copy", "c", false, "copy bookmark to clipboard")
	f.BoolVarP(&cfg.Flags.Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVar(&cfg.Flags.QR, "qr", false, "generate qr-code")
	f.BoolVarP(&cfg.Flags.Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&cfg.Flags.Tags, "tag", "t", nil, "list by tag")
	f.BoolVarP(&cfg.Flags.Update, "update", "u", false, "update a bookmarks")

	// Experimental
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&cfg.Flags.Edit, "edit", "e", false, "edit with preferred text editor")
	f.BoolVarP(&cfg.Flags.Status, "status", "s", false, "check bookmarks status")
	f.BoolVarP(&cfg.Flags.Snapshot, "snapshot", "S", false, "metadata from Wayback Machine")
	f.BoolVarP(&cfg.Flags.Export, "export", "E", false, "export as HTML file")

	// Modifiers
	f.IntVarP(&cfg.Flags.Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&cfg.Flags.Tail, "tail", "T", 0, "the <int> last part of bookmarks")
}

// menuForRecords returns a FZF menu for showing records.
func menuForRecords[T bookmark.Bookmark]() *menu.Menu[T] {
	cfg := config.App
	mo := []menu.OptFn{
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(cfg.Cmd + " --name " + cfg.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(),
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
