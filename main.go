package main

import (
	"database/sql"
	"flag"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var (
	menuName    string
	byQuery     string
	jsonFlag    *bool
	testFlag    *bool
	optionsFlag *bool
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	jsonFlag = flag.Bool("json", false, "JSON output")
	testFlag = flag.Bool("test", false, "test mode")
	optionsFlag = flag.Bool("options", false, "show options")
}

func main() {
	flag.Parse()
	loadMenus()
	setupHomeProject()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	dbPath, err := getDBPath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	menuArgs, err := getMenu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	bookmarksRepo := NewSQLiteRepository(db)
	bookmarksRepo.InitDB()

	if *testFlag {
		handleTestMode(menuArgs, bookmarksRepo)
		return
	}

	if *optionsFlag {
		handleOptionsMode(menuArgs)
		return
	}

	bookmarks, err := fetchBookmarks(bookmarksRepo)
	if err != nil {
		log.Fatal(err)
	}

	if *jsonFlag {
		toJSON(&bookmarks)
		return
	}

	b, err := SelectBookmark(menuArgs, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}
	b.CopyToClipboard()
}
