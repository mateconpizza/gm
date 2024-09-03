package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/spinner"
)

var restoreCmd = &cobra.Command{
	Use:    "restore",
	Short:  "restore bookmarks deleted",
	Hidden: true,
	RunE: func(_ *cobra.Command, args []string) error {
		// Read from deleted table
		Cfg.TableMain = Cfg.GetTableDeleted()
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

		bs := slice.New[Bookmark]()
		if err := handleListAndEdit(r, bs, args); err != nil {
			return err
		}

		if bs.Len() == 0 {
			return repo.ErrRecordNoMatch
		}

		return restore(r, bs)
	},
}

func init() {
	restoreCmd.Flags().BoolVarP(&List, "list", "l", false, "list all bookmarks")
	restoreCmd.Flags().
		StringSliceVarP(&Tags, "tags", "t", nil, "filter bookmarks by tag")
	rootCmd.AddCommand(restoreCmd)
}

// handleRestore restores record/s from the deleted table.
func restore(r *Repo, bs *Slice) error {
	// FIX: remove restored records from deleted table.
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
	success := color.Green("successfully").Italic().Bold()
	fmt.Println("bookmark/s restored", success.String())

	return nil
}
