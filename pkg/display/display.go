package display

import (
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

func Select(cmd *cobra.Command, bs *bookmark.Slice) (*bookmark.Slice, error) {
	menuName, err := cmd.Flags().GetString("menu")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if menuName == "" {
		return bs, nil
	}

	m, err := NewMenu(menuName)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return SelectBookmark(m, bs)
}

func SelectBookmark(m *Menu, bs *bookmark.Slice) (*bookmark.Slice, error) {
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
	index := util.FindSelectedIndex(selectedStr, itemsText)

	if index != -1 {
		b := (*bs)[index]
		log.Printf("Selected bookmark:\n%+v", b)

		return bookmark.NewSlice(&b), nil
	}

	return nil, fmt.Errorf("%w: '%s'", errs.ErrItemNotFound, selectedStr)
}
