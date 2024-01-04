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

func handleTerminalSettings() error {
	if terminal.IsRedirected() || colorFlag == "never" {
		terminal.Defaults.Color = false
	}

	width, height, err := terminal.Size()
	if err != nil {
		if errors.Is(err, terminal.ErrNotTTY) {
			return nil
		}
		return fmt.Errorf("getting console size: %w", err)
	}

	if width < terminal.Defaults.MinWidth {
		return fmt.Errorf("%w: %d. Min: %d", terminal.ErrTermWidthTooSmall, width, terminal.Defaults.MinWidth)
	}

	if height < terminal.Defaults.MinHeight {
		return fmt.Errorf("%w: %d. Min: %d", terminal.ErrTermHeightTooSmall, height, terminal.Defaults.MinHeight)
	}

	if width < terminal.Defaults.MaxWidth {
		terminal.Defaults.MaxWidth = width
	}

	return nil
}

// handleAdd adds a new bookmark
func handleAdd(r *bookmark.SQLiteRepository, args []string) error {
	if !addFlag {
		return nil
	}

	if isPiped && len(args) < 2 {
		return fmt.Errorf("%w: URL or tags cannot be empty", bookmark.ErrInvalidInput)
	}

	url := bookmark.HandleURL(&args)
	if r.RecordExists(config.DB.Table.Main, "url", url) {
		return fmt.Errorf("%w", bookmark.ErrBookmarkDuplicate)
	}
	tags := bookmark.HandleTags(&args)
	title := bookmark.HandleTitle(url)
	desc := bookmark.HandleDesc(url)
	b := bookmark.NewBookmark(url, title, tags, desc)

	if !isPiped {
		if err := handleConfirmAndValidation(b); err != nil {
			return fmt.Errorf("handle confirmation and validation: %w", err)
		}
	}

	b, err := r.Create(config.DB.Table.Main, b)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print("\nNew bookmark added successfully with id: ")
	fmt.Println(format.Text(strconv.Itoa(b.ID)).Green().Bold())
	return nil
}

func handleEdition(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error {
	if err := bookmark.EditAndRenderBookmarks(r, bs, forceFlag); err != nil {
		return fmt.Errorf("error during editing: %w", err)
	}

	return nil
}

func handleRemove(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error {
	for {
		util.CleanTerm()
		s := format.Text(fmt.Sprintf("Bookmarks to remove [%d]:\n", len(*bs))).Red()
		printSliceSummary(bs, s.String())

		if forceFlag {
			break
		}

		confirmMsg := format.Text("Confirm?").Red().String()
		proceed, err := confirmRemove(bs, bookmark.EditionSlice, confirmMsg)
		if !errors.Is(err, bookmark.ErrBufferUnchanged) && err != nil {
			return err
		}

		if proceed {
			break
		}
	}

	if len(*bs) == 0 {
		return fmt.Errorf("%w", bookmark.ErrActionAborted)
	}

	if err := r.DeleteAndReorder(bs); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	total := fmt.Sprintf("\n[%d] bookmarks deleted\n", len(*bs))
	deleting := format.Text(total).Red()
	fmt.Println(deleting)

	return nil
}
