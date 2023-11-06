/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"fmt"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/errs"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "edit selected bookmark",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println(errs.ErrNoIDProvided)
			return
		}

		db := initDB()

		bID, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}

		b, err := db.GetRecordByID(bID, "bookmarks")
		if err != nil {
			fmt.Println(err)
			return
		}

		b, err = bookmark.Edit(b)
		if err != nil {
			fmt.Println(err)
			return
		}

		if _, err := db.UpdateRecord(b, "bookmarks"); err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(b.PrettyColorString())
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
