package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

// importRestoreCmd imports/restore bookmarks from deleted table.
var importRestoreCmd = &cobra.Command{
	Use:     "restore",
	Aliases: []string{"deleted"},
	Short:   "import/restore bookmarks from deleted table",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

		// Switch tables and read from deleted table
		t := r.Cfg.Tables
		r.SetMain(t.Deleted)
		r.SetDeleted(t.Main)

		m := menu.New[Bookmark](
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithHeader("select record/s to restore", false),
		)
		if Multiline {
			m.AddOpts(menu.WithMultilineView())
		}
		bs, err := handleData(m, r, args)
		if err != nil {
			return err
		}

		if bs.Empty() {
			return repo.ErrRecordNoMatch
		}

		if Remove {
			return r.DeleteAndReorder(bs, t.Main, t.RecordsTagsDeleted)
		}

		return restore(r, bs)
	},
}

func init() {
	f := importRestoreCmd.Flags()
	f.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
	f.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&Multiline, "multiline", "M", false, "print data in formatted multiline (fzf)")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&Tags, "tags", "t", nil, "filter bookmarks by tag")
	importCmd.AddCommand(importRestoreCmd)
}

// func init() {
// 	f := importRestoreCmd.Flags()
// 	f.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
// 	f.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
// 	f.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
// 	f.BoolVarP(&Multiline, "multiline", "M", false, "print data in formatted multiline (fzf)")
// 	f.StringSliceVarP(&Tags, "tags", "t", nil, "filter bookmarks by tag")
// }

// handleRestore restores record/s from the deleted table.
func restore(r *repo.SQLiteRepository, bs *Slice) error {
	c := color.BrightYellow
	f := frame.New(frame.WithColorBorder(c), frame.WithNoNewLine())
	header := c("Restoring Bookmarks").String()
	f.Header(header).Ln().Ln().Render().Clean()

	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	}))

	prompt := color.BrightYellow("restore").Bold().String()
	if err := handler.Confirmation(t, bs, prompt, c); err != nil {
		return fmt.Errorf("%w", err)
	}

	mesg := color.Yellow("restoring record/s...").String()
	sp := spinner.New(spinner.WithMesg(mesg))
	sp.Start()
	defer sp.Stop()

	ts := r.Cfg.Tables
	if err := r.Restore(ts.Main, ts.Deleted, bs); err != nil {
		t.ClearLine(1)
		return fmt.Errorf("%w", err)
	}

	t.ClearLine(1)
	f = frame.New(frame.WithColorBorder(color.Gray))
	success := color.BrightGreen("Successfully").Italic().String()
	f.Success(success + " bookmark/s restored").Render()

	return nil
}
