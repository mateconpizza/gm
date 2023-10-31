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

	bm "gomarks/pkg/bookmark"
	c "gomarks/pkg/constants"
	"gomarks/pkg/data"
	db "gomarks/pkg/database"
	"gomarks/pkg/info"
	u "gomarks/pkg/util"
)

var (
	// bookmarks
	addFlag     string
	editFlag    bool
	deleteFlag  bool
	tagsFlag    string
	idFlag      int
	listFlag    bool
	queryFilter string
	copyFlag    bool
	openFlag    bool

	// actions
	format      string
	head        int
	tail        int
	pick        string
	menuName    string
	restoreFlag bool

	// app
	verboseFlag bool
	versionFlag bool
	testFlag    bool
	infoFlag    bool
)

func init() {
	flag.StringVar(&addFlag, "add", "", "add a bookmark [format: URL Tags]")
	flag.BoolVar(&editFlag, "edit", false, "edit a bookmark")
	flag.BoolVar(&deleteFlag, "delete", false, "delete a bookmark")
	flag.StringVar(&tagsFlag, "tags", "", "tag a bookmark")
	flag.IntVar(&idFlag, "id", 0, "bookmark id")
	flag.BoolVar(&listFlag, "list", false, "list all bookmarks")
	flag.StringVar(&queryFilter, "query", "", "query to filter bookmarks")
	flag.BoolVar(&copyFlag, "copy", false, "copy a bookmark")
	flag.BoolVar(&openFlag, "open", false, "open bookmark in default browser")

	flag.StringVar(&format, "f", "pretty", "output format [json|pretty|plain]")
	flag.IntVar(&head, "head", 0, "output the first part of bookmarks")
	flag.IntVar(&tail, "tail", 0, "output the last part of bookmarks")
	flag.StringVar(&pick, "pick", "", "pick data [url|title|tags]")
	flag.StringVar(&menuName, "menu", "", "menu mode [dmenu|rofi]")
	flag.BoolVar(&restoreFlag, "restore", false, "restore a bookmark")

	flag.BoolVar(&testFlag, "test", false, "test mode")
	flag.BoolVar(&verboseFlag, "v", false, "enable verbose output")
	flag.BoolVar(&versionFlag, "version", false, "version")
	flag.BoolVar(&infoFlag, "info", false, "show app info")
}

func parseQueryFlag() {
	// Handle 'query' flag
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		queryFilter = args[0]
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

	// Test mode
	if testFlag {
		fmt.Println("Testing...")
		return
	}

	// Print version
	if versionFlag {
		fmt.Printf("%s v%s\n", c.AppName, c.Version)
		return
	}

	// Print info
	if infoFlag {
		fmt.Println(info.AppInfo(r))
	}

	// Set tableName as deleted table for restore
	if restoreFlag {
		tableName = c.DBDeletedTableName
	}

	// By ID, list or query
	bookmarks, err := data.RetrieveBookmarks(r, tableName, queryFilter, idFlag, listFlag)
	if err != nil {
		fmt.Printf("%s: %s\n", c.AppName, err)
		log.Fatal(err)
	}

	// Apply head and tail options
	bookmarks = data.HeadAndTail(&bookmarks, head, tail)

	// Handle pick
	if pick != "" {
		if err = data.PickAttribute(&bookmarks, pick); err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		return
	}

	// Handle menu option
	if menuName != "" {
		var newBookmarks *bm.BookmarkSlice
		newBookmarks, err = data.PickBookmarkWithMenu(&bookmarks, menuName)
		if err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		bookmarks = *newBookmarks
	}

	// Handle add
	if addFlag != "" {
		if err = data.HandleAdd(r, addFlag, tagsFlag, tableName); err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		return
	}

	// Handle edit
	if editFlag {
		if err = data.HandleEdit(r, &bookmarks[0], tableName); err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		return
	}

	// Handle action
	if copyFlag || openFlag {
		err = data.HandleAction(&bookmarks, copyFlag, openFlag)
		if err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		return
	}

	// Handle format
	if format != "" {
		if err = data.HandleFormat(format, &bookmarks); err != nil {
			fmt.Printf("%s: %s\n", c.AppName, err)
			log.Fatal(err)
		}
		return
	}
}
