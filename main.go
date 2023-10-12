package main

import (
	"database/sql"
	"flag"
	"gomarks/database"
	"gomarks/utils"
	"log"
	"strconv"

	"github.com/atotto/clipboard"
	_ "github.com/mattn/go-sqlite3"
)

var menuName string

func LoadMenus() {
	utils.RegisterMenu("dmenu", []string{"dmenu", "-p", "GoMarks>", "-l", "10"})
	utils.RegisterMenu("rofi", []string{
		"rofi", "-dmenu", "-p", "GoMarks>", "-l", "10", "-mesg",
		" > Welcome to GoMarks\n", "-theme-str", "window {width: 75%; height: 55%;}",
		"-kb-custom-1", "Alt-a"})
}

func init() {
	const (
		defaultMenuName = "dmenu"
		usage           = "the name of menu"
	)
	flag.StringVar(&menuName, "menu", defaultMenuName, usage)
	flag.StringVar(&menuName, "m", defaultMenuName, usage+" (shorthand)")
}

func main() {
	flag.Parse()
	LoadMenus()

	menuArgs, err := utils.GetMenu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	db, err := sql.Open("sqlite3", "bookmarks.db")
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

	selectedIDStr := utils.Prompt(menuArgs, &bookmarks)

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
