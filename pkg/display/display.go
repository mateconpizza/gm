package display

import (
	"database/sql"
	"fmt"
	c "gomarks/pkg/constants"
	db "gomarks/pkg/database"
	"gomarks/pkg/menu"
	"gomarks/pkg/scrape"
	"gomarks/pkg/utils"
	"log"
	"strings"
	"time"
)

/* func ShowOptions(m *menu.Menu) (int, error) {
	options := []fmt.Stringer{
		menu.Option{"Add a bookmark"},
		menu.Option{"Edit a bookmark"},
		menu.Option{"Delete a bookmark"},
		menu.Option{"Exit"},
	}
	idx, err := m.Select(options)
	if err != nil {
		log.Fatal(err)
	}
	return idx, nil
} */

func PavelOptions(menuArgs []string) (int, error) {
	optionsMap := make(map[string]interface{})
	optionsMap["Add a bookmark"] = addBookmark
	optionsMap["Edit a bookmark"] = editBookmark
	optionsMap["Delete a bookmark"] = DeleteBookmark
	optionsMap["Exit"] = nil
	return -1, nil
} */

func AddBookmark(r *db.SQLiteRepository, m *menu.Menu) (db.Bookmark, error) {
	currentTime := time.Now()
	currentTimeString := currentTime.Format("2006-01-02 15:04:05")
	url, err := m.Run("")
	if err != nil {
		return db.Bookmark{}, err
	}

	tags, err := m.Run("")
	if err != nil {
		return db.Bookmark{}, err
	}

	s, err := scrape.TitleAndDescription(url)
  if err != nil {
    return db.Bookmark{}, err
  }

	b, err := r.InsertRecord(&db.Bookmark{
		ID:         0,
		URL:        url,
		Title:      db.NullString{NullString: sql.NullString{String: s.Title, Valid: true}},
		Tags:       tags,
		Desc:       db.NullString{NullString: sql.NullString{String: s.Description, Valid: true}},
		Created_at: currentTimeString,
	}, c.DBMainTable)
	if err != nil {
		return db.Bookmark{}, err
	}
	return b, nil
}

func EditBookmark(r *db.SQLiteRepository, m *menu.Menu, b *db.Bookmark) (db.Bookmark, error) {
	m.UpdatePrompt(fmt.Sprintf("Editing ID: %d", b.ID))
	s, err := m.Run(b.String())
	if err != nil {
		return db.Bookmark{}, err
	}
	fmt.Println(s)
	return *b, nil
}

func DeleteBookmark(r *db.SQLiteRepository, m *menu.Menu, b *db.Bookmark) error {
	msg := fmt.Sprintf("Deleting bookmark: %s", b.URL)
	if !m.Confirm(msg, "Are you sure?") {
		return fmt.Errorf("Cancelled")
	}
	err := r.DeleteRecord(b, c.DBMainTable)
	if err != nil {
		return err
	}

	_, err = r.InsertRecord(b, c.DBDeletedTable)
	if err != nil {
		return err
	}

	err = r.ReorderIDs()
	if err != nil {
		return err
	}
	return nil
}

/* func HandleOptionsMode(m *menu.Menu) {
	idx, err := ShowOptions(m)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Selected:", idx)
} */

func HandleTestMode(m *menu.Menu, r *db.SQLiteRepository) {
	fmt.Print("::::::::Test Mode::::::::\n\n")
	a, _ := r.GetRecordByID(1)
	_, err := editBookmark(r, m, a)
	if err != nil {
		log.Fatal(err)
	}
}

func SelectBookmark(m *menu.Menu, bookmarks *[]db.Bookmark) (db.Bookmark, error) {
	var itemsText []string
	m.UpdateMessage(fmt.Sprintf(" Welcome to GoMarks\n Showing (%d) bookmarks", len(*bookmarks)))

	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			u.ShortenString(bm.URL, 80),
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
		return (*bookmarks)[index], nil
	}
	return db.Bookmark{}, fmt.Errorf("item not found: %s", selectedStr)
}
