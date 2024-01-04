package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/terminal"
)

func promptWithOptions(question string, options []string) string {
	p := format.Prompt(question, fmt.Sprintf("[%s]:", strings.Join(options, "/")))
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(p)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return ""
		}

		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		for _, opt := range options {
			if strings.EqualFold(input, opt) || strings.EqualFold(input, opt[:1]) {
				return input
			}
		}

		fmt.Printf("Invalid response. Please enter one of: %s\n", strings.Join(options, ", "))
	}
}

func exampleUsage(l ...string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", config.App.Name, line)
	}

	return s
}

func getDB() (*bookmark.SQLiteRepository, error) {
	r, err := bookmark.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return r, nil
}

func printSliceSummary(bs *[]bookmark.Bookmark, msg string) {
	fmt.Println(msg)
	for _, b := range *bs {
		idStr := fmt.Sprintf("[%d]", b.ID)
		fmt.Printf(
			"  + %s %s\n",
			format.Text(idStr).Gray(),
			format.Text(format.ShortenString(b.Title, terminal.Defaults.MinWidth)).Purple(),
		)
		fmt.Printf("    %s\n", format.Text("tags:", b.Tags).Gray())
		fmt.Printf("    %s\n\n", format.ShortenString(b.URL, terminal.Defaults.MinWidth))
	}
}

func extractIDsFromStr(args []string) ([]int, error) {
	ids := make([]int, 0)

	for _, arg := range strings.Fields(strings.Join(args, " ")) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				return nil, fmt.Errorf("%w: '%s'", bookmark.ErrInvalidRecordID, arg)
			}
			return nil, fmt.Errorf("%w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func getBookmarksFromArgs(r *bookmark.SQLiteRepository, args []string) (*[]bookmark.Bookmark, error) {
	ids, err := extractIDsFromStr(args)
	if !errors.Is(err, bookmark.ErrInvalidRecordID) && err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	bookmarks, err := r.GetByIDList(config.DB.Table.Main, ids)
	if err != nil {
		return nil, fmt.Errorf("records from args: %w", err)
	}

	return bookmarks, nil
}

// confirmRemove prompts the user to confirm or edit the given bookmark slice.
func confirmRemove(bs *[]bookmark.Bookmark, editFn bookmark.EditFn, question string) (bool, error) {
	// TODO: use this in bookmark single edition
	if isPiped {
		return false, fmt.Errorf(
			"%w: input from pipe is not supported yet. use with --force",
			bookmark.ErrActionAborted,
		)
	}

	options := []string{"Yes", "No", "Edit"}
	option := promptWithOptions(question, options)

	switch option {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, fmt.Errorf("%w", bookmark.ErrActionAborted)
	case "e", "edit":
		if err := editFn(bs); err != nil {
			return false, fmt.Errorf("%w", err)
		}
		return false, nil
	}

	return false, fmt.Errorf("%w", bookmark.ErrActionAborted)
}

func parseBookmarksAndExit(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) {
	actions := map[bool]func(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error{
		statusFlag:  handleStatus,
		editionFlag: handleEdition,
		removeFlag:  handleRemove,
	}

	if action, ok := actions[true]; ok {
		logErrAndExit(action(r, bs))
		os.Exit(0)
	}
}

func parseArgsAndExit(r *bookmark.SQLiteRepository) {
	if versionFlag {
		config.Version()
		os.Exit(0)
	}

	if infoFlag {
		handleAppInfo(r)
		os.Exit(0)
	}
}

func logErrAndExit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}
}

func setLoggingLevel(verboseFlag *bool) {
	if *verboseFlag {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose mode")

		return
	}

	silentLogger := log.New(io.Discard, "", 0)
	log.SetOutput(silentLogger.Writer())
}
