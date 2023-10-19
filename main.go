package main

import (
	"flag"
	"fmt"
	"log"
)

var (
	menuName    string
	byQuery     string
	jsonFlag    *bool
	testFlag    *bool
	optionsFlag *bool
	deleteFlag  *bool
	dropDB      *bool
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	jsonFlag = flag.Bool("json", false, "JSON output")
	testFlag = flag.Bool("test", false, "test mode")
	optionsFlag = flag.Bool("options", false, "show options")
	deleteFlag = flag.Bool("delete", false, "delete a bookmark")
	dropDB = flag.Bool("drop", false, "drop the database")
}

func main() {
	flag.Parse()
	Menus.Load()

	// Set up the home project
	setupHomeProject()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	menu, err := Menus.Get(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

	r := getDB()
	defer r.db.Close()

	if *dropDB {
		r.dropDB()
		return
	}

	if *optionsFlag {
		handleOptionsMode(&menu)
		return
	}

	bookmarks, err := fetchBookmarks(r)
	if err != nil {
		log.Fatal(err)
	}

	if *jsonFlag {
		jsonString := toJSON(&bookmarks)
		fmt.Println(jsonString)
		return
	}

	selectedBookmark, err := SelectBookmark(&menu, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}

	if *deleteFlag {
		if err := deleteBookmark(r, &menu, &selectedBookmark); err != nil {
			log.Fatal(err)
		}
		return
	}

	selectedBookmark.CopyToClipboard()
}
