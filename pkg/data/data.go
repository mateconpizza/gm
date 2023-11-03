package data

import (
	"fmt"
	"math"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	"gomarks/pkg/menu"
)

func QueryAndList(
	r *database.SQLiteRepository,
	byQuery string,
	listFlag bool,
	tableName string,
) (*bookmark.BookmarkSlice, error) {
	var bs *bookmark.BookmarkSlice
	var err error

	if listFlag {
		if bs, err = r.GetRecordsAll(tableName); err != nil {
			return nil, err
		}
	}

	if byQuery != "" {
		if bs, err = r.GetRecordsByQuery(byQuery, tableName); err != nil {
			return nil, err
		}
	}
	return bs, nil
}

func HeadAndTail(bs *bookmark.BookmarkSlice, head, tail int) error {
	// FIX: DRY with 'no bookmarks selected'
	if head > 0 {
		if bs == nil {
			return fmt.Errorf("no bookmarks selected")
		}
		head = int(math.Min(float64(head), float64(len(*bs))))
		*bs = (*bs)[:head]
	}

	if tail > 0 {
		if bs == nil {
			return fmt.Errorf("no bookmarks selected")
		}
		tail = int(math.Min(float64(tail), float64(len(*bs))))
		*bs = (*bs)[len(*bs)-tail:]
	}
	return nil
}

func RetrieveBookmarks(
	r *database.SQLiteRepository,
	tableName *string,
	byQuery *string,
	id int,
	listFlag *bool,
	incomplete bool,
) (*bookmark.BookmarkSlice, error) {
	if id != 0 {
		b, err := r.GetRecordByID(id, *tableName)
		return &bookmark.BookmarkSlice{*b}, err
	}
	if incomplete {
		bs, err := r.GetRecordsWithoutTitleorDesc(*tableName)
		if err != nil {
			return nil, err
		}
		return bs, nil
	}
	return QueryAndList(r, *byQuery, *listFlag, *tableName)
}

func HandleFormat(f string, bs *bookmark.BookmarkSlice) error {
	/* if bs == nil {
		return fmt.Errorf("no bookmarks selected")
	} */
	switch f {
	case "json":
		j := bookmark.ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bs {
			fmt.Println(b.PrettyColorString())
		}
	case "plain":
		for _, b := range *bs {
			fmt.Println(b)
		}
	default:
		return fmt.Errorf("invalid output format: %s", f)
	}
	return nil
}

func PickAttribute(bs *bookmark.BookmarkSlice, s string) error {
	if bs == nil {
		return fmt.Errorf("no bookmarks found")
	}
	for _, b := range *bs {
		switch s {
		case "url":
			fmt.Println(b.URL)
		case "title":
			if b.Title.String != "" {
				fmt.Println(b.Title.String)
			}
		case "tags":
			fmt.Println(b.Tags)
		default:
			return fmt.Errorf("oneline option not found '%s'", s)
		}
	}
	return nil
}

func PickBookmarkWithMenu(bs *bookmark.BookmarkSlice, s string) error {
	if s == "" {
		return nil
	}
	m := menu.New(s)
	b, err := display.SelectBookmark(&m, bs)
	if err != nil {
		return err
	}
	*bs = bookmark.BookmarkSlice{b}
	return nil
}

func FetchBookmarks(
	r *database.SQLiteRepository,
	byQuery, t string,
) (*bookmark.BookmarkSlice, error) {
	var bs *bookmark.BookmarkSlice
	var err error

	switch {
	case byQuery != "":
		bs, err = r.GetRecordsByQuery(byQuery, t)
	default:
		bs, err = r.GetRecordsAll(t)
	}
	return bs, err
}

func HandleEdit(r *database.SQLiteRepository, bs *bookmark.BookmarkSlice, t string) error {
	if bs == nil || len(*bs) == 0 {
		return fmt.Errorf("no bookmarks selected for editing")
	}

	for _, b := range *bs {
		editedBookmark, err := bookmark.Edit(&b)
		if err != nil {
			return fmt.Errorf("bookmark %w", err)
		}

		if _, err := r.UpdateRecord(editedBookmark, t); err != nil {
			return fmt.Errorf("editing bookmark %w", err)
		}
	}
	return nil
}

func HandleAction(bmarks *bookmark.BookmarkSlice, c, o bool) error {
	if len(*bmarks) == 0 {
		return fmt.Errorf("no bookmarks found")
	}
	if c {
		(*bmarks)[0].CopyToClipboard()
	}
	if o {
		fmt.Println("Not implemented yet")
		fmt.Println((*bmarks)[0])
	}
	return nil
}

func HandleAdd(r *database.SQLiteRepository, url, tags, tableName string) error {
	if url == "" {
		return fmt.Errorf("URL is empty")
	}
	if tags == "" {
		return fmt.Errorf("TAGs is empty")
	}
	if r.RecordExists(url, tableName) {
		return fmt.Errorf("bookmark already exists")
	}
	b, err := bookmark.Add(url, tags)
	if err != nil {
		return err
	}
	_, err = r.InsertRecord(b, tableName)
	if err != nil {
		return err
	}
	return nil
}
