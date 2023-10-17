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

func ShowOptions(m *Menu) (int, error) {
	options := []fmt.Stringer{
		Option{"Add a bookmark"},
		Option{"Edit a bookmark"},
		Option{"Delete a bookmark"},
		Option{"Exit"},
	}
	idx, err := Select(m, options)
	if err != nil {
		log.Fatal(err)
	}
	return idx, nil
}

func PavelOptions(menuArgs []string) (int, error) {
	optionsMap := make(map[string]interface{})
	optionsMap["Add a bookmark"] = addBookmark
	optionsMap["Edit a bookmark"] = editBookmark
	optionsMap["Delete a bookmark"] = deleteBookmark
	optionsMap["Exit"] = nil
	return -1, nil
}

func addBookmark(r *SQLiteRepository, m *Menu, b *Bookmark) (Bookmark, error) {
	return Bookmark{}, nil
}

func editBookmark(r *SQLiteRepository, m *Menu, b *Bookmark) (Bookmark, error) {
	return Bookmark{}, nil
}

func deleteBookmark(r *SQLiteRepository, m *Menu, b *Bookmark) error {
	msg := fmt.Sprintf("Deleting bookmark: %s", b.URL)
	if !Confirm(m, msg, "Are you sure?") {
		return fmt.Errorf("Cancelled")
	}
  err := r.RemoveRecord(b)
  if err != nil {
    return err
  }
	return nil
}

func handleOptionsMode(m *Menu) {
	idx, err := ShowOptions(m)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Selected:", idx)
}

func handleTestMode(m *Menu, r *SQLiteRepository) {
	fmt.Println("Test mode")
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

func SelectBookmark(m *Menu, bookmarks *[]Bookmark) (Bookmark, error) {
	var itemsText []string
	m.UpdateMessage(fmt.Sprintf(" Welcome to GoMarks\n Showing (%d) bookmarks", len(*bookmarks)))

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
	output, err := executeCommand(m, itemsString)
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
