package main

import (
	"fmt"
	"log"
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
