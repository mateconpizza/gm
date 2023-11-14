/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var deleteExamples = []string{"delete\n", "delete <id>\n", "delete <query>"}

const maxLen = 80

var deleteCmd = &cobra.Command{
	Use:          "delete",
	Short:        "delete a bookmark by query",
	Example:      exampleUsage(deleteExamples),
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("fetching records: %w", err)
		}

		format.CmdTitle("delete mode")

		bFound := fmt.Sprintf("[%d] bookmarks found\n", bs.Len())
		bf := color.Colorize(bFound, color.Red)
		fmt.Println(bf)

		toDel, err := parseSliceDel(*bs)
		if err != nil {
			return fmt.Errorf("parsing slice: %w", err)
		}

		if err = deleteAndReorder(r, &toDel); err != nil {
			return fmt.Errorf("deleting and reordering records: %w", err)
		}

		total := fmt.Sprintf("[%d] bookmarks deleted.\n", toDel.Len())
		deleting := color.Colorize(total, color.Red)
		fmt.Printf("%s%s\n", color.Bold, deleting)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func deleteAndReorder(r *database.SQLiteRepository, toDel *bookmark.Slice) error {
	if err := r.DeleteRecordsBulk(constants.DBMainTableName, toDel.IDs()); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	if err := r.ReorderIDs(constants.DBMainTableName); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	return nil
}

func getRecords(r *database.SQLiteRepository, args []string) (*bookmark.Slice, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%w", errs.ErrNoIDorQueryPrivided)
	}

	queryOrID := args[0]

	if id, err := strconv.Atoi(queryOrID); err == nil {
		b, err := r.GetRecordByID(constants.DBMainTableName, id)
		if err != nil {
			return nil, fmt.Errorf("getting record by id '%d': %w", id, err)
		}
		return bookmark.NewSlice(b), nil
	}

	bs, err := r.GetRecordsByQuery(constants.DBMainTableName, queryOrID)
	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", queryOrID, err)
	}
	return bs, nil
}

func parseSliceDel(bs bookmark.Slice) (bookmark.Slice, error) {
	if bs.Len() == 0 {
		return nil, fmt.Errorf("%w", errs.ErrBookmarkNotSelected)
	}

	var toDel bookmark.Slice

	for i, b := range bs {
		fmt.Println(b.String())

		deletePrompt := fmt.Sprintf("Delete bookmark [%d/%d]?", i+1, bs.Len())
		confirm := util.Confirm(deletePrompt)

		if confirm {
			toDel = append(toDel, b)
		}

		if bs.Len() > 1 && confirm {
			fmt.Println(color.Colorize("Added to delete queue", color.Red))
		}
		fmt.Println()
	}

	if toDel.Len() == 0 {
		return nil, fmt.Errorf("slice to delete: %w", errs.ErrBookmarkNotSelected)
	}

	if toDel.Len() > 1 {
		d := fmt.Sprintf("Bookmarks to delete [%d]", toDel.Len())
		fmt.Println(color.ColorizeBold(d, color.Red))
		printSliceSummary(&toDel)

		if confirm := util.Confirm("Are you sure?"); !confirm {
			return nil, fmt.Errorf("%w", errs.ErrActionAborted)
		}
	}

	return toDel, nil
}
