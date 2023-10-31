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
) (bookmark.BookmarkSlice, error) {
	var bookmarks bookmark.BookmarkSlice
	var err error

	if byQuery != "" {
		if bookmarks, err = r.GetRecordsByQuery(byQuery, tableName); err != nil {
			return nil, err
		}
	}

	if listFlag {
		if bookmarks, err = r.GetRecordsAll(tableName); err != nil {
			return nil, err
		}
	}
	return bookmarks, nil
}

func HeadAndTail(bookmarks *bookmark.BookmarkSlice, head, tail int) {
	if head > 0 {
		head = int(math.Min(float64(head), float64(len(*bookmarks))))
		*bookmarks = (*bookmarks)[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(len(*bookmarks))))
		*bookmarks = (*bookmarks)[len(*bookmarks)-tail:]
	}
}

func RetrieveBookmarks(
	r *database.SQLiteRepository,
	tableName *string,
	byQuery *string,
	idFlag int,
	listFlag *bool,
) (bookmark.BookmarkSlice, error) {
	if idFlag != 0 {
		b, err := r.GetRecordByID(idFlag, *tableName)
		return bookmark.BookmarkSlice{b}, err
	}
	return QueryAndList(r, *byQuery, *listFlag, *tableName)
}

func HandleFormat(f string, bookmarks *bookmark.BookmarkSlice) error {
	switch f {
	case "json":
		j := bookmark.ToJSON(bookmarks)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bookmarks {
			fmt.Println(b.PrettyColorString())
		}
	case "plain":
		for _, b := range *bookmarks {
			fmt.Println(b)
		}
	default:
		return fmt.Errorf("invalid output format: %s", f)
	}
	return nil
}

func PickAttribute(bmarks *bookmark.BookmarkSlice, s string) error {
	if len(*bmarks) == 0 {
		return fmt.Errorf("no bookmarks found")
	}
	for _, b := range *bmarks {
		switch s {
		case "url":
			fmt.Println(b.URL)
		case "title":
			if b.Title.String != "" {
				fmt.Println(b.Title.String)
			}
		case "tags":
			fmt.Println(b.Tags)
		}
	}
	return nil
}

func PickBookmarkWithMenu(bmarks *bookmark.BookmarkSlice, s string) error {
	m := menu.New(s)
	b, err := display.SelectBookmark(&m, bmarks)
	if err != nil {
		return err
	}
	*bmarks = bookmark.BookmarkSlice{b}
	return nil
}

func FetchBookmarks(
	r *database.SQLiteRepository,
	byQuery, t string,
) (bookmark.BookmarkSlice, error) {
	var bookmarks bookmark.BookmarkSlice
	var err error

	switch {
	case byQuery != "":
		bookmarks, err = r.GetRecordsByQuery(byQuery, t)
	default:
		bookmarks, err = r.GetRecordsAll(t)
	}
	return bookmarks, err
}

func HandleEdit(r *database.SQLiteRepository, bs *bookmark.BookmarkSlice, t string) error {
	if bs == nil || len(*bs) == 0 {
		return fmt.Errorf("no bookmarks selected for editing")
	}

	for _, b := range *bs {
		editedBookmark, err := bookmark.Edit(&b)
		if err != nil {
			return fmt.Errorf("error editing bookmark: %w", err)
		}

		if _, err := r.UpdateRecord(editedBookmark, t); err != nil {
			return fmt.Errorf("error updating bookmark: %w", err)
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
