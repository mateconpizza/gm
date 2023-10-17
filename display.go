package main

import (
	"fmt"
	"log"
	"strings"
)

type Option struct {
	Label string
}

func (o Option) String() string {
	return o.Label
}

func ShowOptions(menuArgs []string) (int, error) {
	options := []fmt.Stringer{
		Option{"Add a bookmark"},
		Option{"Edit a bookmark"},
		Option{"Delete a bookmark"},
		Option{"Exit"},
	}
	idx, err := Select(menuArgs, options)
	if err != nil {
		log.Fatal(err)
	}
	return idx, nil
}

/* func addBookmark() (Bookmark, error) {
	return Bookmark{}, nil
}

func editBookmark() (Bookmark, error) {
	return Bookmark{}, nil
}

func deleteBookmark() (Bookmark, error) {
	return Bookmark{}, nil
} */

func handleOptionsMode(menuArgs []string) {
}

func handleTestMode(menuArgs []string, bookmarksRepo *SQLiteRepository) {
}

func fetchBookmarks(bookmarksRepo *SQLiteRepository) ([]Bookmark, error) {
	var bookmarks []Bookmark
	var err error

	if byQuery != "" {
		bookmarks, err = bookmarksRepo.GetRecordsByQuery(byQuery)
		if err != nil {
			return nil, err
		}
	} else {
		bookmarks, err = bookmarksRepo.GetRecordsAll()
		if err != nil {
			return nil, err
		}
	}

	return bookmarks, nil
}

func SelectBookmark(menuArgs []string, bookmarks *[]Bookmark) (Bookmark, error) {
	var itemsText []string
	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			shortenString(bm.URL, 80),
			bm.Tags,
		)
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")
	output, err := executeCommand(menuArgs, itemsString)
	if err != nil {
		log.Fatal(err)
	}

	selectedStr := strings.Trim(output, "\n")
	index := findSelectedIndex(selectedStr, itemsText)
	if index != -1 {
		return (*bookmarks)[index], nil
	}
	return Bookmark{}, fmt.Errorf("item not found: %s", selectedStr)
}
