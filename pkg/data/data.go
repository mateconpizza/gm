package data

import (
	"fmt"
	"math"

	db "gomarks/pkg/database"
	"gomarks/pkg/display"
	m "gomarks/pkg/menu"
)

func QueryAndList(
	r *db.SQLiteRepository,
	byQuery string,
	listFlag bool,
	tableName string,
) ([]db.Bookmark, error) {
	var bookmarks []db.Bookmark
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

func HeadAndTail(bookmarks []db.Bookmark, head, tail int) []db.Bookmark {
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
) ([]db.Bookmark, error) {
	if idFlag != 0 {
		bookmark, err := r.GetRecordByID(idFlag, tableName)
		return []db.Bookmark{bookmark}, err
	}
	return QueryAndList(r, byQuery, listFlag, tableName)
}

func HandleFormat(f string, bookmarks []db.Bookmark) error {
	switch f {
	case "json":
		j := db.ToJSON(&bookmarks)
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

func PickAttribute(bmarks []db.Bookmark, s string) error {
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

func PickBookmarkWithMenu(bmarks []db.Bookmark, s string) ([]db.Bookmark, error) {
	menu := m.New(s)

	bm, err := display.SelectBookmark(&menu, &bmarks)
	if err != nil {
		return bmarks, err
	}
	return []db.Bookmark{bm}, nil
}
