/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"errors"
	"fmt"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

// TODO:
// - [ ] On `edition mode` create some kind of break statement in buffer
// OR
// - [ ] Find a way to display all records in a buffer and diff changes (like `delete`)
// reference: https://github.com/sergi/go-diff

const tooManyRecords = 8

var editCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit selected bookmark",
	Example: exampleUsage("edit <id>\n", "edit <id id id>\n", "edit <query>"),
	RunE: func(_ *cobra.Command, args []string) error {
		util.ReadInputFromPipe(&args)

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("error getting records: %w", err)
		}

		filteredBs, err := handleHeadAndTail(bs)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := editAndDisplayBookmarks(&filteredBs); err != nil {
			return fmt.Errorf("error during editing: %w", err)
		}

		for _, b := range filteredBs {
			tempB := b
			if _, err := r.Update(config.DB.Table.Main, &tempB); err != nil {
				return fmt.Errorf("error updating records: %w", err)
			}
		}

		return nil
	},
}

func editAndDisplayBookmarks(bs *[]bookmark.Bookmark) error {
	// FIX: split into more functions
	bookmarksToUpdate := []bookmark.Bookmark{}

	if len(*bs) > tooManyRecords {
		return fmt.Errorf("%w: %d. Max: %d", bookmark.ErrTooManyRecords, len(*bs), tooManyRecords)
	}

	for _, b := range *bs {
		tempB := b

		fmt.Printf("%s: bookmark [%d]: ", config.App.Name, tempB.ID)

		editedContent, err := bookmark.EditBuffer(tempB.Buffer())
		if errors.Is(err, bookmark.ErrBufferUnchanged) {
			fmt.Printf("%s\n", format.Text("unchanged").Yellow().Bold())
			bookmark.RemoveItemByID(bs, b.ID)
			continue
		} else if err != nil {
			return fmt.Errorf("error editing bookmark: %w", err)
		}

		tempContent := strings.Split(string(editedContent), "\n")
		if err := bookmark.ValidateBookmarkBuffer(tempContent); err != nil {
			return fmt.Errorf("error validating bookmark buffer: %w", err)
		}

		tb := bookmark.ParseTempBookmark(tempContent)
		bookmark.FetchTitleAndDescription(tb)

		fmt.Printf("%s\n", format.Text("updated").Blue().Bold())

		b.Update(tb.URL, tb.Title, tb.Tags, tb.Desc)
		bookmarksToUpdate = append(bookmarksToUpdate, b)
		*bs = bookmarksToUpdate
	}

	return nil
}

func init() {
	rootCmd.AddCommand(editCmd)
}
