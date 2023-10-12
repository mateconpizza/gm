package main

import (
	"database/sql"
	"flag"
	"gomarks/constants"
	"gomarks/database"
	"gomarks/utils"
	"log"
	"strconv"

	"github.com/atotto/clipboard"
	_ "github.com/mattn/go-sqlite3"
)

var menuName string

func init() {
	const (
		defaultMenuName = "rofi"
		usage           = "the name of menu"
	)
	flag.StringVar(&menuName, "menu", defaultMenuName, usage)
	flag.StringVar(&menuName, "m", defaultMenuName, usage+" (shorthand)")
}

func main() {
	flag.Parse()
	utils.LoadMenus()
	constants.SetupProject()
	dbPath, err := constants.GetDatabasePath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	menuArgs, err := utils.GetMenu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	bookmarksRepository := database.NewSQLiteRepository(db)
	bookmarks, err := bookmarksRepository.GetRecordsAll()
	if err != nil {
		log.Fatal("Error getting bookmarks:", err)
		return
	}

	selectedIDStr, err := utils.Prompt(menuArgs, &bookmarks)
	if err != nil {
		return
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
		log.Fatal("Text copied to clipboard:", bookmark.URL)
	}
}
