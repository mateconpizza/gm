package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "restore deleted bookmarks",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return verifyDatabase(Cfg)
	},
	RunE: func(_ *cobra.Command, args []string) error {
		// Read from deleted table
		Cfg.TableMain = Cfg.TableDeleted
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

		// FIX: respect DRY (check out root.go)
		bs := slice.New[Bookmark]()
		if err := handleRecords(r, bs, args); err != nil {
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
	// TODO: remove restored records from deleted table.
	prompt := color.BrightYellow("restore").Bold().String()
	if err := confirmAction(bs, prompt, color.BrightYellow); err != nil {
		return err
	}

	mesg := color.Yellow("restoring record/s...").String()
	s := spinner.New(spinner.WithMesg(mesg))
	s.Start()

	if err := r.Restore(bs); err != nil {
		return fmt.Errorf("%w: restoring bookmark", err)
	}

	s.Stop()
	success := color.BrightGreen("Successfully").Italic().Bold()
	fmt.Printf("%s bookmark/s restored\n", success)

	return nil
}
