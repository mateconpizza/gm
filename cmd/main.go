package main

// [TODO):
// - [ ] add sub-commands
// - [X] add format option to json, pretty, plain
// - [ ] better module/pkg naming.

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	c "gomarks/pkg/constants"
	"gomarks/pkg/data"
	db "gomarks/pkg/database"
	"gomarks/pkg/display"
	u "gomarks/pkg/util"
)

var (
	addFlag     bool
	byQuery     string
	copyFlag    bool
	deleteFlag  bool
	editFlag    bool
	format      string
	head        int
	idFlag      int
	listFlag    bool
	menuName    string
	openFlag    bool
	pick        string
	restoreFlag bool
	tail        int
	testFlag    bool
	verboseFlag bool
	versionFlag bool
)

func init() {
	flag.BoolVar(&addFlag, "add", false, "add a bookmark")
	flag.BoolVar(&copyFlag, "copy", false, "copy a bookmark")
	flag.BoolVar(&editFlag, "edit", false, "edit a bookmark")
	flag.BoolVar(&deleteFlag, "delete", false, "delete a bookmark")
	flag.BoolVar(&listFlag, "list", false, "list all bookmarks")
	flag.BoolVar(&openFlag, "open", false, "open bookmark in default browser")
	flag.BoolVar(&restoreFlag, "restore", false, "restore a bookmark")
	flag.BoolVar(&testFlag, "test", false, "test mode")
	flag.BoolVar(&verboseFlag, "v", false, "enable verbose output")
	flag.BoolVar(&versionFlag, "version", false, "version")
	flag.IntVar(&head, "head", 0, "output the first part of bookmarks")
	flag.IntVar(&idFlag, "id", 0, "bookmark id")
	flag.IntVar(&tail, "tail", 0, "output the last part of bookmarks")
	flag.StringVar(&byQuery, "query", "", "query to filter bookmarks")
	flag.StringVar(&format, "f", "", "output format [json|pretty|plain]")
	flag.StringVar(&menuName, "menu", "", "menu mode [dmenu rofi]")
	flag.StringVar(&pick, "pick", "", "pick data [url|title|tags]")
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

	parseQueryFlag()
	flag.Parse()

	// Set log level
	u.SetLogLevel(verboseFlag)

	// Set up the home project
	u.SetupHomeProject()

	// Connect to the database
	r := db.GetDB()
	defer r.DB.Close()

	// Print version
	if versionFlag {
		fmt.Println(c.Version)
		return
	}

	// Set tableName as deleted table for restore
	if restoreFlag {
		tableName = c.DBDeletedTableName
	}

	// By ID, list or query
	bookmarks, err := data.RetrieveBookmarks(r, tableName, byQuery, idFlag, listFlag)
	if err != nil {
		log.Fatal(err)
	}

	// Apply head and tail options
	bookmarks = data.HeadAndTail(bookmarks, head, tail)

	// Handle pick
	if pick != "" {
		if err = data.PickAttribute(bookmarks, pick); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Handle menu option
	if menuName != "" {
		if bookmarks, err = data.PickBookmarkWithMenu(bookmarks, menuName); err != nil {
			log.Fatal(err)
		}
	}

	// Handle format
	if format != "" {
		if err = data.HandleFormat(format, bookmarks); err != nil {
			log.Fatal(err)
		}
		return
	}

  // Handle edit
  if editFlag {
    if err = data.HandleEdit(r, &bookmarks[0], tableName); err != nil {
      log.Fatal(err)
    }
    return
  }

	// Handle action
	if copyFlag || openFlag {
		err = display.HandleAction(bookmarks, copyFlag, openFlag)
		if err != nil {
			log.Fatal(err)
		}
		return
	}
}
