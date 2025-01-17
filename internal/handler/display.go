package handler

import (
	"fmt"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Print prints the bookmarks in different formats.
func Print(bs *Slice) error {
	n := terminal.MinWidth
	lastIdx := bs.Len() - 1

	bs.ForEachIdx(func(i int, b Bookmark) {
		fmt.Print(bookmark.Frame(&b, n))
		if i != lastIdx {
			fmt.Println()
		}
	})

	return nil
}

// JSON formats the bookmarks in JSON.
func JSON(bs *Slice) error {
	if bs.Empty() {
		fmt.Println(string(format.ToJSON(config.App)))
		return nil
	}

	fmt.Println(string(format.ToJSON(bs.Items())))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs *Slice) error {
	bs.ForEach(func(b Bookmark) {
		fmt.Print(bookmark.Oneline(&b, terminal.MaxWidth))
	})

	return nil
}

// ByField prints the selected field.
func ByField(bs *Slice, f string) error {
	printer := func(b Bookmark) error {
		f, err := b.Field(f)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(f)

		return nil
	}

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
