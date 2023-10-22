package main

// TODO:
// = [ ] add sub-commands
// = [X] add format option to json, pretty, color-pretty

import (
	"flag"
	"fmt"
	"gomarks/pkg/cli"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	m "gomarks/pkg/menu"
	u "gomarks/pkg/utils"
	"log"
	"os"
)

var (
	addFlag     bool
	byQuery     string
	byTag       string
	deleteFlag  bool
	format      string
	limit       int
	menuName    string
	testFlag    bool
	verboseFlag bool
)

func init() {
	flag.BoolVar(&addFlag, "add", false, "add a bookmark")
	flag.BoolVar(&deleteFlag, "delete", false, "delete a bookmark")
	flag.BoolVar(&testFlag, "test", false, "test mode")
	flag.BoolVar(&verboseFlag, "v", false, "enable verbose output")
	flag.IntVar(&limit, "limit", 0, "limit number of bookmarks")
	flag.StringVar(&byQuery, "query", "", "query to filter bookmarks")
	flag.StringVar(&byTag, "tag", "", "filter bookmarks by tag")
	flag.StringVar(&format, "f", "", "output format [json|pretty|color-pretty]")
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
}

func parseAndExit(r *database.SQLiteRepository, flags *flag.FlagSet, menu *m.Menu) {
	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if addFlag {
		b, err := display.AddBookmark(r, menu)
		if err != nil {
			log.Fatal(err)
		}
		j := database.ToJSON(&[]database.Bookmark{b})
		fmt.Println(j)
		return
	}
}

func getMenu(s string) m.Menu {
	mc := make(m.MenuCollection)
	mc.Load()
	menu, err := mc.Get(menuName)
	if err != nil {
		log.Fatal("Error getting menu:", err)
	}
	return menu
}

func main() {
	tableName := constants.DBMainTableName
	var bookmarks []database.Bookmark
	var err error

	fmt.Printf("Args: %s\n\n", os.Args)
	flag.Parse()

	// Set log level
	u.SetLogLevel(verboseFlag)

	// Set up the home project
	u.SetupHomeProject()

	// Load menus
	menu := getMenu(menuName)

	r := database.GetDB()
	defer r.DB.Close()

	parseAndExit(r, flag.CommandLine, &menu)

	bookmarks, err = database.FetchBookmarks(r, byQuery, byTag, tableName)
	if err != nil {
		log.Fatal(err)
	}

	if limit > 0 {
		if len(bookmarks) > limit {
			bookmarks = bookmarks[:limit]
		}
	}

	if format != "" {
		if err := cli.HandleFormat(format, bookmarks); err != nil {
			log.Fatal(err)
		}
		return
	}

	selectedBookmark, err := display.SelectBookmark(&menu, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}

	if deleteFlag {
		if err := display.DeleteBookmark(r, &menu, &selectedBookmark); err != nil {
			log.Fatal(err)
		}
		return
	}
	selectedBookmark.CopyToClipboard()
}
