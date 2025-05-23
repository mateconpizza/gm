package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/terminal"
)

// recordsCmd is the main command and entrypoint.
var recordsCmd = &cobra.Command{
	Use:     "records",
	Aliases: []string{"r", "items"},
	Short:   "Records management",
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		return handler.CheckDBNotEncrypted()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		terminal.ReadPipedInput(&args)
		bs, err := handler.Data(cmd, handler.MenuForRecords[Bookmark](cmd), r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if bs.Empty() {
			return repo.ErrRecordNotFound
		}
		// actions
		switch {
		case Status:
			return handler.CheckStatus(bs)
		case Remove:
			return handler.Remove(r, bs)
		case Edit:
			return handler.Edition(r, bs)
		case Copy:
			return handler.Copy(bs)
		case Open && !QR:
			return handler.Open(bs)
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
	rootCmd.AddCommand(recordsCmd)
}
