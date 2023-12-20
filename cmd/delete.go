// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "del",
	Short:   "delete a bookmark by query",
	Long:    "delete a bookmark by query or id",
	Example: exampleUsage("del <id>\n", "del <query>\n", "del <id id id>"),
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
			s := format.Text(fmt.Sprintf("Bookmarks to delete [%d]:", len(*bs))).Red()
			printSliceSummary(bs, s.String())

			confirmMsg := format.Text("Confirm?").Red().String()
			proceed, err = confirmDeletion(bs, bookmark.EditionSlice, confirmMsg)
			if !errors.Is(err, bookmark.ErrBufferUnchanged) && err != nil {
				return err
			}

			if proceed {
				break
			}
		}

		if len(*bs) == 0 {
			return fmt.Errorf("%w", bookmark.ErrActionAborted)
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
func deleteAndReorder(r *bookmark.SQLiteRepository, toDel *[]bookmark.Bookmark) error {
	if err := r.DeleteBulk(config.DB.Table.Main, bookmark.ExtractIDs(toDel)); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	if err := r.ReorderIDs(config.DB.Table.Main); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	if err := r.CreateBulk(config.DB.Table.Deleted, toDel); err != nil {
		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
