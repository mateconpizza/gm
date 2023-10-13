package main

import (
	"database/sql"
	"flag"
	"log"
	"strconv"

	"github.com/atotto/clipboard"
	_ "github.com/mattn/go-sqlite3"
)

var (
	menuName      string
	byQuery       string
	jsonFlag      *bool
	bookmarks     []Bookmark
	selectedIDStr string
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	jsonFlag = flag.Bool("json", false, "JSON output")
}

func main() {
	flag.Parse()
	LoadMenus()
	SetupHomeProject()

	dbPath, err := GetDatabasePath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	menuArgs, err := Menu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	bookmarksRepository := NewSQLiteRepository(db)
	bookmarksRepository.InitDB()

	if byQuery != "" {
		bookmarks, err = bookmarksRepository.GetRecordsByQuery(byQuery)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		bookmarks, err = bookmarksRepository.GetRecordsAll()
		if err != nil {
			log.Fatal("Error getting bookmarks:", err)
		}
	}

	if *jsonFlag {
		ToJSON(&bookmarks)
		return
	} else {
		selectedIDStr, err = Prompt(menuArgs, &bookmarks)
		if err != nil {
			return
		}
	}

	bookmark_id, err := strconv.Atoi(selectedIDStr)
	if err != nil {
		log.Fatal("Error converting string to int:", err)
	}

	bookmark, err := bookmarksRepository.GetRecordByID(bookmark_id)
	if err != nil {
		log.Fatal("Error getting bookmark:", err)
	}

	err = clipboard.WriteAll(bookmark.URL)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	} else {
		log.Println("Text copied to clipboard:", bookmark.URL)
	}
}
