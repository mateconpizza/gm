package display

import (
	"fmt"
	"log"
	"strings"

	bm "gomarks/pkg/bookmark"
	c "gomarks/pkg/constants"
	db "gomarks/pkg/database"
	"gomarks/pkg/menu"
	u "gomarks/pkg/util"
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

func DeleteBookmark(r *db.SQLiteRepository, m *menu.Menu, b *bm.Bookmark) error {
	msg := fmt.Sprintf("Deleting bookmark: %s", b.URL)
	if !m.Confirm(msg, "Are you sure?") {
		return fmt.Errorf("Cancelled")
	}

	err := r.DeleteRecord(b, c.DBMainTableName)
	if err != nil {
		return err
	}

	_, err = r.InsertRecord(b, c.DBDeletedTableName)
	if err != nil {
		return err
	}

	err = r.ReorderIDs()
	if err != nil {
		return err
	}

	return nil
}

func SelectBookmark(m *menu.Menu, bookmarks *bm.Slice) (bm.Bookmark, error) {
	maxLen := 80
	itemsText := make([]string, 0, len(*bookmarks))

	msg := fmt.Sprintf(" Welcome to GoMarks\n Showing (%d) bookmarks", len(*bookmarks))
	m.UpdateMessage(msg)
	log.Printf("Selecting bookmark from %d bookmarks\n", len(*bookmarks))

	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			u.ShortenString(bm.URL, maxLen),
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
	index := u.FindSelectedIndex(selectedStr, itemsText)

	if index != -1 {
		b := (*bookmarks)[index]
		log.Printf("Selected bookmark:\n%+v", b)

		return b, nil
	}

	return bm.Bookmark{}, fmt.Errorf("item not found: '%s'", selectedStr)
}
