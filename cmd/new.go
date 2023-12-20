// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:     "new",
	Short:   "add a new bookmark",
	Long:    "add a new bookmark and fetch title and description",
	Example: exampleUsage("new\n", "new <url>\n", "new <url> <tags>"),
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		url := bookmark.HandleURL(&args)
		if r.RecordExists(config.DB.Table.Main, "url", url) {
			return fmt.Errorf("%w", bookmark.ErrBookmarkDuplicate)
		}

		tags := bookmark.HandleTags(&args)
		title := bookmark.HandleTitle(url)
		desc := bookmark.HandleDesc(url)

		b := bookmark.NewBookmark(url, title, tags, desc)

		if err = handleConfirmAndValidation(b); err != nil {
			return fmt.Errorf("handle confirmation and validation: %w", err)
		}

		b, err = r.Create(config.DB.Table.Main, b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Print("\nNew bookmark added successfully with id: ")
		fmt.Println(format.Text(strconv.Itoa(b.ID)).Green().Bold())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

func handleConfirmAndValidation(b *bookmark.Bookmark) error {
	options := []string{"Yes", "No", "Edit"}
	option := promptWithOptions("Save bookmark?", options)

	switch option {
	case "n":
		return fmt.Errorf("%w", bookmark.ErrActionAborted)
	case "e":
		editedContent, err := bookmark.Edit(b.Buffer())

		if errors.Is(err, bookmark.ErrBufferUnchanged) {
			return nil
		} else if err != nil {
			return fmt.Errorf("%w", err)
		}

		editedBookmark := bookmark.ParseTempBookmark(strings.Split(string(editedContent), "\n"))
		bookmark.FetchTitleAndDescription(editedBookmark)

		b.Update(editedBookmark.URL, editedBookmark.Title, editedBookmark.Tags, editedBookmark.Desc)

		if err := bookmark.Validate(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}
