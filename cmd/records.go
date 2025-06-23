package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

func init() {
	initRecordFlags(recordsCmd)

	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.json, "json", "j", false, "output tags+count in JSON format")
	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.list, "list", "l", false, "list all tags")

	recordsCmd.AddCommand(recordsTagsCmd)
	Root.AddCommand(recordsCmd)
}

type tagsFlagType struct {
	json bool
	list bool
}

var (
	// recordsCmd records management.
	// main command.
	recordsCmd = &cobra.Command{
		Use:               "records",
		Aliases:           []string{"r"},
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
			switch {
			case tagsFlags.json:
				return printer.JSONTags(config.App.DBPath)
			case tagsFlags.list:
				return printer.TagsList(config.App.DBPath)
			}

			return cmd.Usage()
		},
	}
)

// recordsCmd is the main command and entrypoint.
func recordsCmdFunc(cmd *cobra.Command, args []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(menuForRecords[bookmark.Bookmark](cmd), r, args)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if bs.Empty() {
		return db.ErrRecordNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	cfg := config.App

	switch {
	case cfg.Flags.Status:
		return handler.CheckStatus(c, bs)
	case cfg.Flags.Remove:
		return handler.Remove(c, r, bs)
	case cfg.Flags.Edit:
		return handler.EditSlice(c, r, bs)
	case cfg.Flags.Update:
		return handler.UpdateSlice(c, r, bs)
	case cfg.Flags.Copy:
		return handler.Copy(bs)
	case cfg.Flags.Open && !cfg.Flags.QR:
		return handler.Open(c, r, bs)
	}

	switch {
	case cfg.Flags.Field != "":
		return printer.ByField(bs, cfg.Flags.Field)
	case cfg.Flags.QR:
		return handler.QR(bs, cfg.Flags.Open)
	case cfg.Flags.JSON:
		return printer.JSONRecordSlice(bs)
	case cfg.Flags.Oneline:
		return printer.Oneline(bs)
	default:
		return printer.RecordSlice(bs)
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

	// Actions
	f.BoolVarP(&cfg.Flags.Copy, "copy", "c", false, "copy bookmark to clipboard")
	f.BoolVarP(&cfg.Flags.Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&cfg.Flags.QR, "qr", "q", false, "generate qr-code")
	f.BoolVarP(&cfg.Flags.Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&cfg.Flags.Tags, "tag", "t", nil, "list by tag")
	f.BoolVarP(&cfg.Flags.Update, "update", "u", false, "update a bookmarks")

	// Experimental
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&cfg.Flags.Edit, "edit", "e", false, "edit with preferred text editor")
	f.BoolVarP(&cfg.Flags.Status, "status", "s", false, "check bookmarks status")

	// Modifiers
	f.IntVarP(&cfg.Flags.Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&cfg.Flags.Tail, "tail", "T", 0, "the <int> last part of bookmarks")
}

// menuForRecords returns a FZF menu for showing records.
func menuForRecords[T comparable](cmd *cobra.Command) *menu.Menu[T] {
	mo := []menu.OptFn{
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(config.App.Cmd + " --name " + config.App.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(),
			config.FzfKeybindOpen(),
			config.FzfKeybindQR(),
			config.FzfKeybindOpenQR(),
			config.FzfKeybindYank(),
		),
	}

	if multi, _ := cmd.Flags().GetBool("multiline"); multi {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
