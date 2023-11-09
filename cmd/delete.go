/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"

	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/errs"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var deleteExamples = []string{"delete\n", "delete <id>\n", "delete <query>"}

var deleteCmd = &cobra.Command{
	Use:          "delete",
	Short:        "delete a bookmark by query",
	Example:      exampleUsage(deleteExamples),
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ids := make([]int, 0)
		urls := make([]string, 0)
		cmdTitle("delete mode")

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		q := args[0]
		bs, err := r.GetRecordsByQuery(constants.DBMainTableName, q)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Printf("%s%s[%d] bookmarks found%s\n\n", color.Bold, color.Red, bs.Len(), color.Reset)

		for _, b := range *bs {
			fmt.Println(b.PrettyColorString())

			confirm := util.ConfirmChanges("Delete bookmark?")

			if confirm {
				ids = append(ids, b.ID)
				urls = append(urls, b.URL)
			}
			fmt.Println("")
		}

		if len(ids) == 0 {
			return fmt.Errorf("%w", errs.ErrBookmarkNotSelected)
		}

		fmt.Printf("%s%sBookmarks to delete:%s\n", color.Bold, color.Red, color.Reset)
		for _, url := range urls {
			fmt.Printf("\t+ %s\n", url)
		}

		confirm := util.ConfirmChanges(
			fmt.Sprintf("Deleting [%d] bookmarks, are you sure?", len(ids)),
		)

		if !confirm {
			return fmt.Errorf("%w", errs.ErrActionAborted)
		}

		err = r.DeleteRecordsBulk(constants.DBMainTableName, ids)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		err = r.ReorderIDs(constants.DBMainTableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
