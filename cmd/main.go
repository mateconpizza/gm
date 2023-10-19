package main

import (
	"flag"
	"fmt"
	"log"
  "gomarks/pkg/database"
  "gomarks/pkg/utils"
  "gomarks/pkg/menu"
  "gomarks/pkg/display"
)

var (
	menuName    string
	byQuery     string
	jsonFlag    bool
	testFlag    *bool
	optionsFlag *bool
	deleteFlag  *bool
	addFlag     string
	dropDB      *bool
	migrateDB   *bool
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	flag.BoolVar(&jsonFlag, "json", false, "JSON output")
  flag.StringVar(&addFlag, "add", "", "add a bookmark")
	testFlag = flag.Bool("test", false, "test mode")
	optionsFlag = flag.Bool("options", false, "show options")
	deleteFlag = flag.Bool("delete", false, "delete a bookmark")
	dropDB = flag.Bool("drop", false, "drop the database")
	migrateDB = flag.Bool("migrate", false, "migrate database")
}

func main() {
	flag.Parse()
	menu.Menus.Load()

	// Set up the home project
	utils.SetupHomeProject()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	menu, err := menu.Menus.Get(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}

  r := database.GetDB()
	defer r.DB.Close()

	if *dropDB {
		r.HandleDropDB()
		return
	}

	if *testFlag {
		display.HandleTestMode(&menu, r)
		return
	}

	if *migrateDB {
		database.MigrateData(r)
		return
	}

  if addFlag != "" {
    // handleAddMode(&menu, r, addFlag)
    fmt.Println("Add mode: ", addFlag)
    return
  }

	if *optionsFlag {
		// display.HandleOptionsMode(&menu)
		return
	}

	bookmarks, err := database.FetchBookmarks(r, byQuery)
	if err != nil {
		log.Fatal(err)
	}

	if jsonFlag {
		jsonString := database.ToJSON(&bookmarks)
		fmt.Println(jsonString)
		return
	}

	selectedBookmark, err := display.SelectBookmark(&menu, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}

	if *deleteFlag {
		if err := display.DeleteBookmark(r, &menu, &selectedBookmark); err != nil {
			log.Fatal(err)
		}
		return
	}

	selectedBookmark.CopyToClipboard()
}
