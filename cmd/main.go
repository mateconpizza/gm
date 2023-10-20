package main

import (
	"flag"
	"fmt"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	m "gomarks/pkg/menu"
	"gomarks/pkg/utils"
	"log"
)

var (
	menuName    string
	byQuery     string
	jsonFlag    bool
	testFlag    *bool
	optionsFlag *bool
	deleteFlag  *bool
	addFlag     *bool
	dropDB      *bool
	migrateData *bool
	verboseFlag bool
)

func init() {
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
	flag.StringVar(&byQuery, "q", "", "query to filter bookmarks")
	flag.BoolVar(&jsonFlag, "json", false, "JSON output")
	flag.BoolVar(&verboseFlag, "v", false, "Enable verbose output")
	addFlag = flag.Bool("add", false, "add a bookmark")
	testFlag = flag.Bool("test", false, "test mode")
	optionsFlag = flag.Bool("options", false, "show options")
	deleteFlag = flag.Bool("delete", false, "delete a bookmark")
	dropDB = flag.Bool("drop", false, "drop the database")
	migrateData = flag.Bool("migrate", false, "migrate data")
}

func main() {
	var tableName string = constants.DBMainTableName
	flag.Parse()

	// Set log level
	utils.SetLogLevel(verboseFlag)

	// Load menus
	m.Menus.Load()

	// Set up the home project
	utils.SetupHomeProject()

	menu, err := m.Menus.Get(menuName)
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

	if *addFlag {
		b, err := display.AddBookmark(r, &menu)
		if err != nil {
			log.Fatal(err)
		}
		j := database.ToJSON(&[]database.Bookmark{b})
		fmt.Println(j)
		return
	}

	if *optionsFlag {
		// display.HandleOptionsMode(&menu)
		return
	}

	if *migrateData {
		tableName = constants.DBDeletedTableName
	}

	bookmarks, err := database.FetchBookmarks(r, byQuery, tableName)
	if err != nil {
		log.Fatal(err)
	}

	if jsonFlag {
		j := database.ToJSON(&bookmarks)
		fmt.Println(j)
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
