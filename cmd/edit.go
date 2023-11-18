/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit selected bookmark",
	Example: exampleUsage([]string{"edit <id>\n", "edit <query>"}),
	RunE: func(cmd *cobra.Command, args []string) error {
		format.CmdTitle("edition mode.")

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if bs.Len() > 1 {
			bFound := fmt.Sprintf("[%d] bookmarks found\n", bs.Len())
			bf := color.Colorize(bFound, color.Blue)
			fmt.Println(bf)

			for i, b := range *bs {
				tempB := b
				fmt.Println()
				fmt.Println(b.String())

				editPrompt := fmt.Sprintf("Edit bookmark [%d/%d]?", i+1, bs.Len())
				if confirm := util.Confirm(editPrompt); confirm {
					_, err = bookmark.Edit(&tempB)
					if err != nil {
						return fmt.Errorf("%w", err)
					}
				}
			}
		}

		id := (*bs)[0].ID

		b, err := r.GetRecordByID(config.DB.MainTable, id)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		b, err = bookmark.Edit(b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if _, err := r.UpdateRecord(config.DB.MainTable, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(cmd, &bookmark.Slice{*b}); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
