package display

import (
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

func Select(cmd *cobra.Command, bs *[]bookmark.Bookmark) ([]bookmark.Bookmark, error) {
	menuName, err := cmd.Flags().GetString("menu")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if menuName == "" {
		return *bs, nil
	}

	m, err := NewMenu(menuName)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return SelectBookmark(m, bs)
}

func SelectBookmark(m *Menu, bs *[]bookmark.Bookmark) ([]bookmark.Bookmark, error) {
	maxLen := 80
	itemsText := make([]string, 0, len(*bs))

	msg := fmt.Sprintf(" Welcome to GoMarks\n Showing (%d) bookmarks", len(*bs))
	m.UpdateMessage(msg)
	log.Printf("Selecting bookmark from %d bookmarks\n", len(*bs))

	for _, bm := range *bs {
		itemText := fmt.Sprintf(
			"%-4d %-*s %-10s",
			bm.ID,
			maxLen,
			format.ShortenString(bm.URL, maxLen),
			bm.Tags,
		)
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")

	output, err := m.Run(itemsString)
	if err != nil {
		return nil, fmt.Errorf("error running menu: %w", err)
	}

	selectedStr := strings.Trim(output, "\n")
	index := findSelectedIndex(selectedStr, itemsText)

	if index != -1 {
		b := (*bs)[index]
		log.Printf("Selected bookmark:\n%+v", b)

		return []bookmark.Bookmark{b}, nil
	}

	return nil, fmt.Errorf("%w: '%s'", bookmark.ErrRecordNotFound, selectedStr)
}
