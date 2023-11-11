package display

import (
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/errs"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"
)

func SelectBookmark(m *menu.Menu, bookmarks *bookmark.Slice) (*bookmark.Bookmark, error) {
	maxLen := 80
	itemsText := make([]string, 0, len(*bookmarks))

	msg := fmt.Sprintf(" Welcome to GoMarks\n Showing (%d) bookmarks", len(*bookmarks))
	m.UpdateMessage(msg)
	log.Printf("Selecting bookmark from %d bookmarks\n", len(*bookmarks))

	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			util.ShortenString(bm.URL, maxLen),
			bm.Tags,
		)
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")

	output, err := m.Run(itemsString)
	if err != nil {
		return &bookmark.Bookmark{}, fmt.Errorf("error running menu: %w", err)
	}

	selectedStr := strings.Trim(output, "\n")
	index := util.FindSelectedIndex(selectedStr, itemsText)

	if index != -1 {
		b := (*bookmarks)[index]
		log.Printf("Selected bookmark:\n%+v", b)

		return &b, nil
	}

	return &bookmark.Bookmark{}, fmt.Errorf("%w: '%s'", errs.ErrItemNotFound, selectedStr)
}
