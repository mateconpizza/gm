package main

// [TODO):
// = [ ] add sub-commands
// = [X] add format option to json, pretty, color-pretty

import (
	"flag"
	"fmt"
	"gomarks/pkg/cli"
	c "gomarks/pkg/constants"
	db "gomarks/pkg/database"
	"gomarks/pkg/display"
	m "gomarks/pkg/menu"
	u "gomarks/pkg/utils"
	"log"
	"math"
	"os"
	"strings"
)

var (
	addFlag     bool
	byQuery     string
	deleteFlag  bool
	format      string
	head        int
	tail        int
	menuName    string
	testFlag    bool
	verboseFlag bool
	versionFlag bool
)

func init() {
	flag.BoolVar(&addFlag, "add", false, "add a bookmark")
	flag.BoolVar(&deleteFlag, "delete", false, "delete a bookmark")
	flag.BoolVar(&testFlag, "test", false, "test mode")
	flag.BoolVar(&verboseFlag, "v", false, "enable verbose output")
	flag.BoolVar(&versionFlag, "version", false, "version")
	flag.IntVar(&head, "head", 0, "output the first part of bookmarks")
	flag.IntVar(&tail, "tail", 0, "output the last part of bookmarks")
	flag.StringVar(&byQuery, "query", "", "query to filter bookmarks")
	flag.StringVar(&format, "f", "", "output format [json|pretty|color-pretty]")
	flag.StringVar(&menuName, "m", "rofi", "name of the menu [dmenu rofi]")
}

func parseAndExit(r *db.SQLiteRepository, flags *flag.FlagSet, menu *m.Menu) {
	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if testFlag {
		display.HandleTestMode(menu, r)
		os.Exit(0)
	}

	if addFlag {
		b, err := display.AddBookmark(r, menu)
		if err != nil {
			log.Fatal(err)
		}
		j := db.ToJSON(&[]db.Bookmark{b})
		fmt.Println(j)
		os.Exit(0)
	}

	if versionFlag {
		fmt.Println(cli.Version)
		os.Exit(0)
	}
}

func parseQueryFlag() {
	// Handle 'query' flag
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		byQuery = args[0]
		args = args[1:]
	}
	os.Args = append([]string{os.Args[0]}, args...)
}

func main() {
	tableName := c.DBMainTableName
	var bookmarks []db.Bookmark
	var err error

	parseQueryFlag()
	flag.Parse()

	// Set log level
	u.SetLogLevel(verboseFlag)

	// Set up the home project
	u.SetupHomeProject()

	// Load menus
	menu := m.GetMenu(menuName)

	r := db.GetDB()
	defer r.DB.Close()

	parseAndExit(r, flag.CommandLine, &menu)

	if bookmarks, err = db.FetchBookmarks(r, byQuery, tableName); err != nil {
		log.Fatal(err)
	}

	if head > 0 {
		head = int(math.Min(float64(head), float64(len(bookmarks))))
		bookmarks = bookmarks[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(len(bookmarks))))
		bookmarks = bookmarks[len(bookmarks)-tail:]
	}

	if format != "" {
		if err := cli.HandleFormat(format, bookmarks); err != nil {
			log.Fatal(err)
		}
		return
	}

	bm, err := display.SelectBookmark(&menu, &bookmarks)
	if err != nil {
		log.Fatal(err)
	}

	if deleteFlag {
		if err := display.DeleteBookmark(r, &menu, &bm); err != nil {
			log.Fatal(err)
		}
		return
	}
	bm.CopyToClipboard()
}
