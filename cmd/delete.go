// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/app"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "del",
	Short:   "delete a bookmark by query",
	Long:    "delete a bookmark by query or id",
	Example: exampleUsage("delete <id>\n", "delete <query>\n", "delete <id id id>"),
	RunE: func(_ *cobra.Command, args []string) error {
		var proceed bool

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		for {
			util.CleanTerm()
			s := format.Text(fmt.Sprintf("Bookmarks to delete [%d]:", len(*bs))).Red().Bold()
			printSliceSummary(bs, s.String())

			proceed, err = confirmProceed(bs, bookmark.EditionSlice)

			if !errors.Is(err, bookmark.ErrBufferUnchanged) && err != nil {
				return err
			}

			if proceed {
				break
			}
		}

		if len(*bs) == 0 {
			return fmt.Errorf("%w", errs.ErrActionAborted)
		}

		if err = deleteAndReorder(r, bs); err != nil {
			return fmt.Errorf("deleting and reordering records: %w", err)
		}

		total := fmt.Sprintf("\n[%d] bookmarks deleted\n", len(*bs))
		deleting := format.Text(total).Red()
		fmt.Println(deleting)

		return nil
	},
}

// deleteAndReorder deletes the specified bookmarks from the database and
// reorders the remaining IDs.
func deleteAndReorder(r *database.SQLiteRepository, toDel *bookmark.Slice) error {
	if err := r.DeleteRecordsBulk(app.DB.Table.Main, toDel.IDs()); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	if err := r.ReorderIDs(app.DB.Table.Main); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	if err := r.InsertRecordsBulk(app.DB.Table.Deleted, toDel); err != nil {
		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
