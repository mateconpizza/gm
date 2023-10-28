package data

import (
	"fmt"
	"math"

	bm "gomarks/pkg/bookmark"
	db "gomarks/pkg/database"
	"gomarks/pkg/display"
	m "gomarks/pkg/menu"
)

func QueryAndList(
	r *db.SQLiteRepository,
	byQuery string,
	listFlag bool,
	tableName string,
) (bm.BookmarkSlice, error) {
	var bookmarks bm.BookmarkSlice
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

func HeadAndTail(bookmarks bm.BookmarkSlice, head, tail int) bm.BookmarkSlice {
	if head > 0 {
		head = int(math.Min(float64(head), float64(len(bookmarks))))
		bookmarks = bookmarks[:head]
	}

	if tail > 0 {
		tail = int(math.Min(float64(tail), float64(len(bookmarks))))
		bookmarks = bookmarks[len(bookmarks)-tail:]
	}
	return bookmarks
}

func RetrieveBookmarks(
	r *db.SQLiteRepository,
	tableName string,
	byQuery string,
	idFlag int,
	listFlag bool,
) (bm.BookmarkSlice, error) {
	if idFlag != 0 {
		bookmark, err := r.GetRecordByID(idFlag, tableName)
		return bm.BookmarkSlice{bookmark}, err
	}
	return QueryAndList(r, byQuery, listFlag, tableName)
}

func HandleFormat(f string, bookmarks bm.BookmarkSlice) error {
	switch f {
	case "json":
		j := bm.ToJSON(&bookmarks)
		fmt.Println(j)
	case "pretty":
		for _, b := range bookmarks {
			fmt.Println(b.PrettyColorString())
		}
	case "plain":
		for _, b := range bookmarks {
			fmt.Println(b)
		}
	default:
		return fmt.Errorf("invalid output format: %s", f)
	}
	return nil
}

func PickAttribute(bmarks bm.BookmarkSlice, s string) error {
	if len(bmarks) == 0 {
		return fmt.Errorf("no bookmarks found")
	}
	for _, b := range bmarks {
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

func PickBookmarkWithMenu(bmarks bm.BookmarkSlice, s string) (bm.BookmarkSlice, error) {
	menu := m.New(s)
	b, err := display.SelectBookmark(&menu, &bmarks)
	if err != nil {
		return bmarks, err
	}
	return bm.BookmarkSlice{b}, nil
}

func FetchBookmarks(r *db.SQLiteRepository, byQuery, t string) (bm.BookmarkSlice, error) {
	var bookmarks bm.BookmarkSlice
	var err error

	switch {
	case byQuery != "":
		bookmarks, err = r.GetRecordsByQuery(byQuery, t)
	default:
		bookmarks, err = r.GetRecordsAll(t)
	}
	return bookmarks, err
}
