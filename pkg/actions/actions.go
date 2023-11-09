package actions

import (
	"fmt"
	"math"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	"gomarks/pkg/errs"
	"gomarks/pkg/menu"
)

func QueryAndList(
	r *database.SQLiteRepository,
	byQuery string,
	listFlag bool,
	tableName string,
) (*bookmark.Slice, error) {
	var bs *bookmark.Slice
	var err error

	if listFlag {
		if bs, err = r.GetRecordsAll(tableName); err != nil {
			return nil, fmt.Errorf("%w: on applying query and list", err)
		}
	}

	if byQuery != "" {
		if bs, err = r.GetRecordsByQuery(tableName, byQuery); err != nil {
			return nil, fmt.Errorf("%w: on applying query and list", err)
		}
	}

	return bs, nil
}

func HeadAndTail(bs *bookmark.Slice, head, tail int) error {
	// FIX: DRY with 'no bookmarks selected'
	if head > 0 {
		if bs == nil {
			return errs.ErrBookmarkNotSelected
		}

		head = int(math.Min(float64(head), float64(len(*bs))))
		*bs = (*bs)[:head]
	}

	if tail > 0 {
		if bs == nil {
			return errs.ErrBookmarkNotSelected
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
) (*bookmark.Slice, error) {
	if id != 0 {
		b, err := r.GetRecordByID(*tableName, id)
		if err != nil {
			return nil, fmt.Errorf("%w (retrieving bookmarks)", err)
		}
		return &bookmark.Slice{*b}, nil
	}

	return QueryAndList(r, *byQuery, *listFlag, *tableName)
}

func HandleFormat(f string, bs *bookmark.Slice) error {
	switch f {
	case "json":
		j := bookmark.ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bs {
			fmt.Println(b.PrettyColorString())
		}
		fmt.Printf("%stotal [%d]%s\n", color.Gray, bs.Len(), color.Reset)
	case "plain":
		for _, b := range *bs {
			fmt.Println(b)
		}
	default:
		return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, f)
	}

	return nil
}

func PickAttribute(bs *bookmark.Slice, s string) error {
	if bs == nil {
		return errs.ErrBookmarkNotFound
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
			return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, s)
		}
	}

	return nil
}

func PickBookmarkWithMenu(bs *bookmark.Slice, s string) error {
	if s == "" {
		return nil
	}

	m := menu.New(s)
	b, err := display.SelectBookmark(m, bs)
	if err != nil {
		return fmt.Errorf("%w: picking bookmark with menu", err)
	}

	*bs = bookmark.Slice{b}

	return nil
}

func FetchBookmarks(
	r *database.SQLiteRepository,
	byQuery, t string,
) (*bookmark.Slice, error) {
	var bs *bookmark.Slice

	var err error

	switch {
	case byQuery != "":
		bs, err = r.GetRecordsByQuery(t, byQuery)
	default:
		bs, err = r.GetRecordsAll(t)
	}

	return bs, fmt.Errorf("%w: fetching bookmarks", err)
}

func HandleEdit(r *database.SQLiteRepository, bs *bookmark.Slice, tableName string) error {
	if bs == nil || len(*bs) == 0 {
		return errs.ErrBookmarkNotSelected
	}

	for _, b := range *bs {
		bookmarkToEdit := b
		editedBookmark, err := bookmark.Edit(&bookmarkToEdit)
		if err != nil {
			return fmt.Errorf("bookmark %w", err)
		}

		if _, err := r.UpdateRecord(tableName, editedBookmark); err != nil {
			return fmt.Errorf("editing bookmark %w", err)
		}
	}

	return nil
}

func HandleAction(bmarks *bookmark.Slice, c, o bool) error {
	if len(*bmarks) == 0 {
		return errs.ErrBookmarkNotFound
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

func HandleAdd(r *database.SQLiteRepository, url, tableName string) (*bookmark.Slice, error) {
	fmt.Println("not implemented yet")
	return &bookmark.Slice{}, nil
}

func SelectBookmark(bs *bookmark.Slice) (*bookmark.Bookmark, error) {
	fmt.Println("not implemented yet")
	return &bookmark.Bookmark{}, nil
}
