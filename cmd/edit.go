/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

// TODO:
// - [ ] On `edition` create some kind of break statement in buffer

const tooManyRecords = 5
const maxRecords = 20

var editCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit selected bookmark",
	Example: exampleUsage([]string{"edit <id>\n", "edit <id id id>\n", "edit <query>"}),
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("error getting records: %w", err)
		}

		if len(*bs) > maxRecords {
			return fmt.Errorf("%w for edition: %d", errs.ErrTooManyRecords, len(*bs))
		}

		bsToUpdate, err := editAndDisplayBookmarks(bs)
		if err != nil {
			return fmt.Errorf("error during editing: %w", err)
		}

		if err := r.UpdateRecordsBulk(config.DB.Table.Main, bsToUpdate); err != nil {
			return fmt.Errorf("error updating records: %w", err)
		}

		return nil
	},
}

func editAndDisplayBookmarks(bs *bookmark.Slice) (*bookmark.Slice, error) {
	bookmarksToUpdate := bookmark.Slice{}

	for i, b := range *bs {
		// BUG: I dont know how i feel about this...
		if i+1 > tooManyRecords {
			if !util.Confirm(fmt.Sprintf("Continue editing [%d/%d] ?", i+1, len(*bs))) {
				return nil, fmt.Errorf("%w", errs.ErrActionAborted)
			}
		}

		tempB := b

		fmt.Printf("%s: bookmark [%d]: ", config.App.Name, tempB.ID)

		bookmarkEdited, err := bookmark.Edit(&tempB)
		if err != nil {
			if errors.Is(err, errs.ErrBookmarkUnchanged) {
				fmt.Printf("%s\n", format.Warning("unchanged"))
				continue
			}
			return nil, fmt.Errorf("error editing bookmark: %w", err)
		}

		fmt.Printf("%s\n", format.Info("updated"))
		bookmarksToUpdate.Add(bookmarkEdited)
	}

	return &bookmarksToUpdate, nil
}

func init() {
	rootCmd.AddCommand(editCmd)
}
