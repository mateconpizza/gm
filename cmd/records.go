package cmd

import (
	"fmt"
	"log/slog"

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
		Use:     "records",
		Aliases: []string{"r"},
		Short:   "Records management",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return handler.AssertDatabaseExists(cmd)
		},
		RunE: recordsCmdFunc,
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
	bs, err := handler.Data(cmd, menuForRecords[bookmark.Bookmark](cmd), r, args)
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

	switch {
	case Status:
		return handler.CheckStatus(c, bs)
	case Remove:
		return handler.Remove(c, r, bs)
	case Edit:
		return handler.EditSlice(c, r, bs)
	case Copy:
		return handler.Copy(bs)
	case Open && !QR:
		return handler.Open(r, bs)
	}

	switch {
	case Field != "":
		return printer.ByField(bs, Field)
	case QR:
		return handler.QR(bs, Open)
	case JSON:
		return printer.JSONRecordSlice(bs)
	case Oneline:
		return printer.Oneline(bs)
	default:
		return printer.RecordSlice(bs)
	}
}

func initRecordFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	// Prints
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	f.BoolVarP(&Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	f.StringVarP(&Field, "field", "f", "", "output by field [id|url|title|tags]")

	// Actions
	f.BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	f.BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&QR, "qr", "q", false, "generate qr-code")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&Tags, "tag", "t", nil, "list by tag")

	// Experimental
	f.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&Edit, "edit", "e", false, "edit with preferred text editor")
	f.BoolVarP(&Status, "status", "s", false, "check bookmarks status")

	// Modifiers
	f.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
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
	multi, err := cmd.Flags().GetBool("multiline")
	if err != nil {
		slog.Debug("getting 'Multiline' flag", "error", err.Error())
		multi = false
	}
	if multi {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
