/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:     "new",
	Short:   "add a new bookmark",
	Long:    "add a new bookmark and fetch title and description",
	Example: exampleUsage([]string{"new\n", "new <url>\n", "new <url> <tags>"}),
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		format.CmdTitle("adding a new bookmark")

		url := handleURL(&args)

		if r.RecordExists(config.DB.Table.Main, url) {
			if b, _ := r.GetRecordByURL(config.DB.Table.Main, url); b != nil {
				return fmt.Errorf("%w with id: %d", errs.ErrBookmarkDuplicate, b.ID)
			}
		}

		tags := handleTags(&args)
		title := handleTitle(url)
		desc := handleDesc(url)

		b := bookmark.New(url, title, tags, desc)

		if err = handleConfirmAndValidation(b); err != nil {
			return fmt.Errorf("handle confirmation and validation: %w", err)
		}

		b, err = r.InsertRecord(config.DB.Table.Main, b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(format.Success("\nNew bookmark added successfully with id:", strconv.Itoa(b.ID)))

		return nil
	},
}

func init() {
	var url string
	var tags string
	newCmd.Flags().StringVarP(&url, "url", "u", "", "url for new bookmark")
	newCmd.Flags().StringVarP(&tags, "tags", "t", "", "tags for new bookmark")
	rootCmd.AddCommand(newCmd)
}

func handleConfirmAndValidation(b *bookmark.Bookmark) error {
	options := []string{"Yes", "No", "Edit"}
	o := promptWithOptions("Save bookmark?", options)
	switch o {
	case "n":
		return fmt.Errorf("%w", errs.ErrActionAborted)
	case "e":
		editedBookmark, err := bookmark.Edit(b)

		if errors.Is(err, errs.ErrBookmarkUnchaged) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := bookmark.Validate(editedBookmark); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}
