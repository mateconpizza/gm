package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"
)

func handleQuery(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%w", bookmark.ErrNoIDorQueryProvided)
	}

	queryOrID, err := util.GetInputFromArgs(args)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return queryOrID, nil
}

func handleFormat(bs *[]bookmark.Bookmark) error {
	if pickerFlag != "" {
		return nil
	}

	if err := bookmark.Format(formatFlag, *bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func handlePicker(bs *[]bookmark.Bookmark) error {
	if pickerFlag == "" {
		return nil
	}

	for _, b := range *bs {
		switch pickerFlag {
		case "id":
			fmt.Println(b.ID)
		case "url":
			fmt.Println(b.URL)
		case "title":
			fmt.Println(b.Title)
		case "tags":
			fmt.Println(b.Tags)
		default:
			return fmt.Errorf("%w: %s", format.ErrInvalidOption, pickerFlag)
		}
	}

	return nil
}

func handleHeadAndTail(bs *[]bookmark.Bookmark) ([]bookmark.Bookmark, error) {
	newBs := *bs

	if headFlag < 0 || tailFlag < 0 {
		return nil, fmt.Errorf("%w: %d %d", format.ErrInvalidOption, headFlag, tailFlag)
	}

	if headFlag > 0 {
		headFlag = int(math.Min(float64(headFlag), float64(len(newBs))))
		newBs = newBs[:headFlag]
	}

	if tailFlag > 0 {
		tailFlag = int(math.Min(float64(tailFlag), float64(len(newBs))))
		newBs = newBs[len(newBs)-tailFlag:]
	}

	return newBs, nil
}

// handleGetRecords retrieves records from the database based on either an ID or a query string.
func handleGetRecords(r *bookmark.SQLiteRepository, args []string) (*[]bookmark.Bookmark, error) {
	// FIX: split into more functions
	ids, err := extractIDs(args)
	if !errors.Is(err, bookmark.ErrInvalidRecordID) && err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if len(ids) > 0 {
		bookmarks := make([]bookmark.Bookmark, 0, len(ids))

		for _, id := range ids {
			b, err := r.GetByID(config.DB.Table.Main, id)
			if err != nil {
				return nil, fmt.Errorf("getting record by ID '%d': %w", id, err)
			}
			bookmarks = append(bookmarks, *b)
		}

		return &bookmarks, nil
	}

	queryOrID, err := handleQuery(args)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	var id int
	var b *bookmark.Bookmark

	if id, err = strconv.Atoi(queryOrID); err == nil {
		b, err = r.GetByID(config.DB.Table.Main, id)
		if err != nil {
			return nil, fmt.Errorf("getting record by id '%d': %w", id, err)
		}
		return &[]bookmark.Bookmark{*b}, nil
	}

	bs, err := r.GetByQuery(config.DB.Table.Main, queryOrID)
	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", queryOrID, err)
	}
	return bs, nil
}

func handleInfoFlag(r *bookmark.SQLiteRepository) {
	lastMainID := r.GetMaxID(config.DB.Table.Main)
	lastDeletedID := r.GetMaxID(config.DB.Table.Deleted)

	if formatFlag == "json" {
		config.DB.Records.Main = lastMainID
		config.DB.Records.Deleted = lastDeletedID
		fmt.Println(format.ToJSON(config.AppConf))
	} else {
		fmt.Println(config.ShowInfo(lastMainID, lastDeletedID))
	}
}

func handleTermOptions() error {
	if util.IsOutputRedirected() || colorFlag == "never" {
		format.WithColor = false
	}

	width, height, err := util.GetConsoleSize()
	if errors.Is(err, config.ErrNotTTY) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting console size: %w", err)
	}

	if width < config.Term.MinWidth {
		return fmt.Errorf("%w: %d. Min: %d", config.ErrTermWidthTooSmall, width, config.Term.MinWidth)
	}

	if height < config.Term.MinHeight {
		return fmt.Errorf("%w: %d. Min: %d", config.ErrTermHeightTooSmall, height, config.Term.MinHeight)
	}

	if width < config.Term.MaxWidth {
		config.Term.MaxWidth = width
	}

	return nil
}

func parseBookmarksAndExit(bs *[]bookmark.Bookmark) {
	if statusFlag {
		logErrAndExit(handleCheckStatus(bs))
		os.Exit(0)
	}

	if editionFlag {
		logErrAndExit(handleEdition(bs))
		os.Exit(0)
	}
}

func parseArgsAndExit(r *bookmark.SQLiteRepository) {
	if versionFlag {
		name := format.Text(config.App.Data.Title).Blue().Bold()
		fmt.Printf("%s v%s\n", name, config.App.Version)
		os.Exit(0)
	}

	if infoFlag {
		handleInfoFlag(r)
		os.Exit(0)
	}
}

func logErrAndExit(err error) {
	if err != nil {
		fmt.Printf("%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}
}

func handleCheckStatus(bs *[]bookmark.Bookmark) error {
	if len(*bs) == 0 {
		return bookmark.ErrBookmarkNotSelected
	}

	if err := bookmark.CheckStatus(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func handleEdition(bs *[]bookmark.Bookmark) error {
	if len(*bs) == 0 {
		return bookmark.ErrBookmarkNotSelected
	}

	if err := editAndDisplayBookmarks(bs); err != nil {
		return fmt.Errorf("error during editing: %w", err)
	}

	s := format.Text("\nexperimental:").Red().Bold()
	m := format.Text("nothing was updated").Blue()
	fmt.Println(s, m)

	return nil
}
