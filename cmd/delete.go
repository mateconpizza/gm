/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/errs"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var deleteExamples = []string{"delete\n", "delete <id>\n", "delete <query>"}

func parseSliceDel(bs bookmark.Slice) ([]int, error) {
	if bs.Len() == 0 {
		return nil, fmt.Errorf("%w", errs.ErrBookmarkNotSelected)
	}

	var toDel bookmark.Slice

	for i, b := range bs {
		fmt.Println(b.PrettyColorString())

		confirm := util.Confirm(fmt.Sprintf("Delete bookmark [%d/%d]?", i+1, bs.Len()))

		if toDel.Len() > 1 && confirm {
			fmt.Printf("%sAdded to delete queue%s\n", color.Red, color.Reset)
			toDel = append(toDel, b)
		}
		fmt.Println("")
	}

	if toDel.Len() == 0 {
		return nil, fmt.Errorf("%w", errs.ErrBookmarkNotSelected)
	}

	if toDel.Len() > 1 {
		fmt.Printf("%s%sBookmarks to delete:%s\n", color.Bold, color.Red, color.Reset)
		for _, b := range toDel {
			fmt.Printf("\t+ [%d] %s\n", b.ID, b.URL)
		}

		if confirm := util.Confirm(fmt.Sprintf("Deleting [%d] bookmark/s, are you sure?", toDel.Len())); !confirm {
			return nil, fmt.Errorf("%w", errs.ErrActionAborted)
		}
	}

	return toDel.IDs(), nil
}

var deleteCmd = &cobra.Command{
	Use:          "delete",
	Short:        "delete a bookmark by query",
	Example:      exampleUsage(deleteExamples),
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("%w", errs.ErrNoIDorQueryPrivided)
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		cmdTitle("delete mode")

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		q := args[0]
		bs, err := r.GetRecordsByQuery(constants.DBMainTableName, q)
		if err != nil {
			return fmt.Errorf("getting records by query '%s': %w", q, err)
		}

		fmt.Printf("%s%s[%d] bookmarks found%s\n\n", color.Bold, color.Red, bs.Len(), color.Reset)

		toDel, err := parseSliceDel(*bs)
		if err != nil {
			return fmt.Errorf("parsing slice: %w", err)
		}

		if err = r.DeleteRecordsBulk(constants.DBMainTableName, toDel); err != nil {
			return fmt.Errorf("deleting records in bulk: %w", err)
		}

		if err := r.ReorderIDs(constants.DBMainTableName); err != nil {
			return fmt.Errorf("reordering ids: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
