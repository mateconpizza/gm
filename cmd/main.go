package main

// [TODO):
// - [ ] add sub-commands (maybe use Cobra!)
// - [X] add format option to json, pretty, plain
// - [ ] better module/pkg naming.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gomarks/pkg/constants"
	"gomarks/pkg/data"
	"gomarks/pkg/database"
	"gomarks/pkg/info"
	"gomarks/pkg/util"
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
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		queryFilter = args[0]
		args = args[1:]
	}
	os.Args = append([]string{os.Args[0]}, args...)
}

func main() {
	tableName := constants.DBMainTableName

	parseQueryFlag()
	flag.Parse()

	if versionFlag {
		fmt.Printf("%s v%s\n", constants.AppName, constants.Version)
		return
	}

	util.SetLogLevel(verboseFlag)
	util.SetupHomeProject()
	r := database.GetDB()
	defer r.DB.Close()

	// Test mode
	if testFlag {
		fmt.Println("Testing...")
		return
	}

	// Print info
	if infoFlag {
		fmt.Println(info.AppInfo(r))
	}

	// Set tableName as deleted table for restore
	if restoreFlag {
		tableName = constants.DBDeletedTableName
	}

	// By ID, list or query
	bookmarks, err := data.RetrieveBookmarks(r, &tableName, &queryFilter, idFlag, &listFlag)
	if err != nil {
		util.PrintErrMsg(err.Error(), verboseFlag)
	}

	// Apply head and tail options
	data.HeadAndTail(&bookmarks, head, tail)

	// Handle pick
	if pick != "" {
		if err = data.PickAttribute(&bookmarks, pick); err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
		return
	}

	// Handle menu option
	if menuName != "" {
		err = data.PickBookmarkWithMenu(&bookmarks, menuName)
		if err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
	}

	// Handle add
	if addFlag != "" {
		if err = data.HandleAdd(r, addFlag, tagsFlag, tableName); err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
		return
	}

	// Handle edit
	if editFlag {
		if err = data.HandleEdit(r, &bookmarks, tableName); err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
		return
	}

	// Handle action
	if copyFlag || openFlag {
		err = data.HandleAction(&bookmarks, copyFlag, openFlag)
		if err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
		return
	}

	// Handle format
	if format != "" {
		if err = data.HandleFormat(format, &bookmarks); err != nil {
			util.PrintErrMsg(err.Error(), verboseFlag)
		}
		return
	}
}
