package main

import (
	"flag"
	"log"
)

var (
	menuName    string
	byQuery     string
	jsonFlag    *bool
	testFlag    *bool
	optionsFlag *bool
	deleteFlag  *bool
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	jsonFlag = flag.Bool("json", false, "JSON output")
	testFlag = flag.Bool("test", false, "test mode")
	optionsFlag = flag.Bool("options", false, "show options")
	deleteFlag = flag.Bool("delete", false, "delete a bookmark")
}

func main() {
	flag.Parse()
	loadMenus()
	setupHomeProject()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	menu, err := getMenu(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	bookmarksRepo := getDB()
	defer bookmarksRepo.db.Close()

	if *testFlag {
		handleTestMode(&menu, bookmarksRepo)
		return
	}

	if *migrateDB {
		MigrateDB(bookmarksRepo)
		return
	}

	if *optionsFlag {
		handleOptionsMode(&menu)
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

	b, err := SelectBookmark(&menu, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}
	b.CopyToClipboard()
}
