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

func HandleFormat(f string, bs *bookmark.Slice) error {
	switch f {
	case "json":
		j := bookmark.ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bs {
			fmt.Println(b.String())
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
			if b.Title != "" {
				fmt.Println(b.Title)
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

	m, err := menu.New(s)
	if err != nil {
		return fmt.Errorf("picking bookmark with menu: %w", err)
	}

	b, err := display.SelectBookmark(m, bs)
	if err != nil {
		return fmt.Errorf("picking bookmark with menu: %w", err)
	}

	*bs = bookmark.Slice{*b}

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

func SelectBookmark(bs *bookmark.Slice) (*bookmark.Bookmark, error) {
	fmt.Println("not implemented yet")
	return &bookmark.Bookmark{}, nil
}
