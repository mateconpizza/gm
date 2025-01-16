package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "restore deleted bookmarks",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		handler.OnSubcommand()
		return verifyDatabase(Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read from deleted table
		Cfg.Tables.Main = Cfg.Tables.Deleted
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

		bs, err := handleData(r, args)
		if err != nil {
			return err
		}
		if bs.Empty() {
			return repo.ErrRecordNoMatch
		}

		return restore(r, bs)
	},
}

func init() {
	restoreCmd.Flags().IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	restoreCmd.Flags().IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
	restoreCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	restoreCmd.Flags().
		BoolVarP(&Multiline, "multiline", "M", false, "print data in formatted multiline (fzf)")
	restoreCmd.Flags().
		StringSliceVarP(&Tags, "tags", "t", nil, "filter bookmarks by tag")
	rootCmd.AddCommand(restoreCmd)
}

// handleRestore restores record/s from the deleted table.
func restore(r *repo.SQLiteRepository, bs *Slice) error {
	c := color.BrightYellow
	f := frame.New(frame.WithColorBorder(c), frame.WithNoNewLine())
	header := c("Restoring Bookmarks").String()
	f.Header(header).Ln().Ln().Render().Clean()

	// TODO?: remove restored records from deleted table.
	prompt := color.BrightYellow("restore").Bold().String()
	if err := handler.Confirmation(bs, prompt, c); err != nil {
		return fmt.Errorf("restore confirmation: %w", err)
	}

	mesg := color.Yellow("restoring record/s...").String()
	s := spinner.New(spinner.WithMesg(mesg))
	s.Start()

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction", err)
	}

	if err := r.Restore(tx, bs); err != nil {
		return fmt.Errorf("%w: restoring bookmark", err)
	}

	s.Stop()

	terminal.ClearLine(1)
	f = frame.New(frame.WithColorBorder(color.Gray))
	success := color.BrightGreen("Successfully").Italic().String()
	f.Success(success + " bookmark/s restored").Render()

	return nil
}
