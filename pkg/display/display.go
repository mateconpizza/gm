package display

import (
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"
)

/* func AddBookmark(r *db.SQLiteRepository, m *menu.Menu) (bm.Bookmark, error) {
	currentTime := time.Now()
	currentTimeString := currentTime.Format("2006-01-02 15:04:05")

	m.UpdatePrompt("Enter URL:")
	url, err := m.Run("")
	if err != nil {
		return bm.Bookmark{}, err
	}

	m.UpdatePrompt("Enter tags:")
	tags, err := m.Run("")
	if err != nil {
		return bm.Bookmark{}, err
	}

	s, err := scrape.TitleAndDescription(url)
	if err != nil {
		return bm.Bookmark{}, err
	}

	b, err := r.InsertRecord(&bm.Bookmark{
		ID:         0,
		URL:        url,
		Title:      bm.NullString{NullString: sql.NullString{String: s.Title, Valid: true}},
		Tags:       tags,
		Desc:       bm.NullString{NullString: sql.NullString{String: s.Description, Valid: true}},
		Created_at: currentTimeString,
	}, c.DBMainTableName)
	if err != nil {
		return bm.Bookmark{}, err
	}
	return b, nil
} */

/* func EditBookmark(r *db.SQLiteRepository, m *menu.Menu, b *bm.Bookmark) (bm.Bookmark, error) {
	m.UpdatePrompt(fmt.Sprintf("Editing ID: %d", b.ID))
	s, err := m.Run(b.String())
	if err != nil {
		return bm.Bookmark{}, err
	}
	fmt.Println(s)
	return *b, nil
} */

func DeleteBookmark(r *database.SQLiteRepository, m *menu.Menu, b *bookmark.Bookmark) error {
	msg := fmt.Sprintf("Deleting bookmark: %s", b.URL)
	if !m.Confirm(msg, "Are you sure?") {
		return errs.ErrActionCancelled
	}

	err := r.DeleteRecord(b, constants.DBMainTableName)
	if err != nil {
		return fmt.Errorf("%w: deleting record with menu", err)
	}

	_, err = r.InsertRecord(b, constants.DBDeletedTableName)
	if err != nil {
		return fmt.Errorf("%w: deleting and inserting record", err)
	}

	err = r.ReorderIDs()
	if err != nil {
		return fmt.Errorf("%w: reordering Ids after deletion", err)
	}

	return nil
}

func SelectBookmark(m *menu.Menu, bookmarks *bookmark.Slice) (bookmark.Bookmark, error) {
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
		log.Fatal(err)
	}

	selectedStr := strings.Trim(output, "\n")
	index := util.FindSelectedIndex(selectedStr, itemsText)

	if index != -1 {
		b := (*bookmarks)[index]
		log.Printf("Selected bookmark:\n%+v", b)

		return b, nil
	}

	return bookmark.Bookmark{}, fmt.Errorf("%w: '%s'", errs.ErrItemNotFound, selectedStr)
}
