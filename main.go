package main

import (
	"database/sql"
	"flag"
	"log"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

var (
	menuName      string
	byQuery       string
	jsonFlag      *bool
	testFlag      *bool
	bookmarks     []Bookmark
	selectedIDStr string
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	jsonFlag = flag.Bool("json", false, "JSON output")
	testFlag = flag.Bool("test", false, "test mode")
}

func main() {
	flag.Parse()
	LoadMenus()
	SetupHomeProject()

	dbPath, err := GetDatabasePath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	bookmarksRepo := NewSQLiteRepository(db)

	if *testFlag {
		return
	}

	bookmarksRepo.InitDB()

	if byQuery != "" {
		bookmarks, err = bookmarksRepo.GetRecordsByQuery(byQuery)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		bookmarks, err = bookmarksRepo.GetRecordsAll()
		if err != nil {
			log.Fatal("Error getting bookmarks:", err)
		}
	}

	if *jsonFlag {
		ToJSON(&bookmarks)
		return
	}

	menuArgs, err := Menu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	selectedIDStr, err = Prompt(menuArgs, &bookmarks)
	if err != nil {
		return
	}

	bookmark_id, err := strconv.Atoi(selectedIDStr)
	if err != nil {
		log.Fatal("Error converting string to int:", err)
	}

	bookmark, err := bookmarksRepo.GetRecordByID(bookmark_id)
	if err != nil {
		log.Fatal("Error getting bookmark:", err)
	}

	CopyToClipboard(bookmark.URL)
}
