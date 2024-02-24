package cmd

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
)

// handleFormat formats the bookmarks
func handleFormat(bs *[]bookmark.Bookmark) error {
	if pickerFlag != "" {
		return nil
	}

	if err := bookmark.Format(formatFlag, *bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handlePicker prints the selected field
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
		case "desc":
			fmt.Println(b.Desc)
		default:
			return fmt.Errorf("%w: %s", format.ErrInvalidOption, pickerFlag)
		}
	}

	return nil
}

// handleHeadAndTail returns a slice of bookmarks with limited elements
func handleHeadAndTail(bs *[]bookmark.Bookmark) ([]bookmark.Bookmark, error) {
	newBs := *bs

	if headFlag < 0 || tailFlag < 0 {
		return nil, fmt.Errorf("%w: %d %d", format.ErrInvalidOption, headFlag, tailFlag)
	}

	// Adjust the slice size based on the provided options
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

// handleFetchRecords retrieves records from the database based on either an ID or a query string.
func handleFetchRecords(r *bookmark.SQLiteRepository, args []string) (*[]bookmark.Bookmark, error) {
	if len(args) == 0 {
		return nil, bookmark.ErrNoQueryProvided
	}

	if tagFlag != "" {
		bs, err := handleByTags(r)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		return bs, nil
	}

	bs, err := getBookmarksFromArgs(r, args)
	if err != nil {
		return nil, err
	}

	if bs != nil {
		return bs, nil
	}

	query := strings.Join(args, "%")
	bs, err = r.GetByQuery(config.DB.Table.Main, query)

	if bs == nil {
		return nil, fmt.Errorf("%w: %s", bookmark.ErrRecordNotFound, strings.Join(args, " "))
	}

	if err != nil {
		return nil, fmt.Errorf("getting records by query '%s': %w", strings.Join(args, " "), err)
	}

	return bs, nil
}

// handleAppInfo prints the app and database info
func handleAppInfo(r *bookmark.SQLiteRepository) {
	lastMainID := r.GetMaxID(config.DB.Table.Main)
	lastDeletedID := r.GetMaxID(config.DB.Table.Deleted)

	if formatFlag == "json" {
		config.DB.Records.Main = lastMainID
		config.DB.Records.Deleted = lastDeletedID
		fmt.Println(string(format.ToJSON(config.AppConf)))
		return
	}

	fmt.Println(config.Info(lastMainID, lastDeletedID))
}

// handleByTags returns a slice of bookmarks based on the provided tags
func handleByTags(r *bookmark.SQLiteRepository) (*[]bookmark.Bookmark, error) {
	if tagFlag == "" {
		return nil, fmt.Errorf("%w: %s", bookmark.ErrInvalidInput, tagFlag)
	}

	tags := strings.Split(tagFlag, " ")
	uniqueMap := make(map[int]bookmark.Bookmark)

	for _, tag := range tags {
		bs, err := r.GetByTags(config.DB.Table.Main, tag)
		if err != nil {
			return nil, fmt.Errorf("getting records by tag '%s': %w", tag, err)
		}

		for _, b := range *bs {
			if _, ok := uniqueMap[b.ID]; ok {
				continue
			}
			uniqueMap[b.ID] = b
		}
	}

	return mapToSlice(uniqueMap), nil
}

func mapToSlice(m map[int]bookmark.Bookmark) *[]bookmark.Bookmark {
	bs := make([]bookmark.Bookmark, 0, len(m))
	for _, b := range m {
		bs = append(bs, b)
	}
	return &bs
}

// handleAdd fetch metadata and adds a new bookmark
func handleAdd(r *bookmark.SQLiteRepository, args []string) error {
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

// handleEdition renders the edition interface
func handleEdition(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error {
	if err := bookmark.EditAndRenderBookmarks(r, bs, forceFlag); err != nil {
		return fmt.Errorf("error during editing: %w", err)
	}

	return nil
}

// handleRemove removes bookmarks
func handleRemove(r *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error {
	for {
		for _, b := range *bs {
			fmt.Println(b.DeleteString())
		}

		if forceFlag {
			break
		}

		msg := fmt.Sprintf("remove %d bookmark/s?", len(*bs))
		confirmMsg := format.Text(msg).Red().String()
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

// handleStatus prints the status code of the bookmark URL
func handleStatus(_ *bookmark.SQLiteRepository, bs *[]bookmark.Bookmark) error {
	if err := bookmark.CheckStatus(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleConfirmAndValidation confirms if the user wants to save the bookmark
func handleConfirmAndValidation(b *bookmark.Bookmark) error {
	// FIX: Split me
	options := []string{"Yes", "No", "Edit"}
	option := promptWithOptions("Save bookmark?", options)

	switch option {
	case "n":
		return fmt.Errorf("%w", bookmark.ErrActionAborted)
	case "e":
		editedContent, err := bookmark.Edit(b.Buffer())
		if err != nil {
			if errors.Is(err, bookmark.ErrBufferUnchanged) {
				return nil
			}
			return fmt.Errorf("%w", err)
		}

		editedBookmark := bookmark.ParseTempBookmark(strings.Split(string(editedContent), "\n"))
		bookmark.FetchTitleAndDescription(editedBookmark)

		b.Update(editedBookmark.URL, editedBookmark.Title, editedBookmark.Tags, editedBookmark.Desc)

		if err := bookmark.Validate(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}
