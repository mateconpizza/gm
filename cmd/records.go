package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

type tagsFlagType struct {
	json bool
	list bool
}

var (
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
				return handler.JSONTags(config.App.DBPath)
			case tagsFlags.list:
				return handler.ListTags(config.App.DBPath)
			}
			return cmd.Usage()
		},
	}
)

// recordsCmd is the main command and entrypoint.
var recordsCmd = &cobra.Command{
	Use:     "records",
	Aliases: []string{"r", "items"},
	Short:   "Records management",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := handler.CheckDBLocked(config.App.DBPath); err != nil {
			return fmt.Errorf("%w", err)
		}

		return handler.ValidateDBExists(config.App.DBPath)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		terminal.ReadPipedInput(&args)
		bs, err := handler.Data(cmd, handler.MenuForRecords[bookmark.Bookmark](cmd), r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if bs.Empty() {
			return db.ErrRecordNotFound
		}
		// actions
		switch {
		case Status:
			return handler.CheckStatus(bs)
		case Remove:
			return handler.Remove(r, bs)
		case Edit:
			return handler.EditSlice(r, bs)
		case Copy:
			return handler.Copy(bs)
		case Open && !QR:
			return handler.Open(r, bs)
		}
		// display
		switch {
		case Field != "":
			return handler.ByField(bs, Field)
		case QR:
			return handler.QR(bs, Open)
		case JSON:
			return handler.JSONSlice(bs)
		case Oneline:
			return handler.Oneline(bs)
		default:
			return handler.Print(bs)
		}
	},
}

func init() {
	rf := recordsCmd.Flags()
	rf.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	rf.BoolVarP(&Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	rf.BoolVarP(&Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	rf.StringVarP(&Field, "field", "f", "", "output by field [id|url|title|tags]")
	// Actions
	rf.BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	rf.BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	rf.BoolVarP(&QR, "qr", "q", false, "generate qr-code")
	rf.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	rf.StringSliceVarP(&Tags, "tag", "t", nil, "list by tag")
	// Experimental
	rf.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	rf.BoolVarP(&Edit, "edit", "e", false, "edit with preferred text editor")
	rf.BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	// Modifiers
	rf.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	rf.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")

	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.json, "json", "j", false, "output tags+count in JSON format")
	recordsTagsCmd.Flags().BoolVarP(&tagsFlags.list, "list", "l", false, "list all tags")

	recordsCmd.AddCommand(recordsTagsCmd)
	rootCmd.AddCommand(recordsCmd)
}
