package handler

import (
	"fmt"
	"log/slog"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
)

// Print prints the bookmarks in a frame format with the given colorscheme.
func Print(bs *Slice) error {
	s := config.App.Colorscheme
	lastIdx := bs.Len() - 1
	cs, ok := color.Schemes[s]
	if !ok {
		return fmt.Errorf("%w: %q", color.ErrColorSchemeUnknown, s)
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	bs.ForEachIdx(func(i int, b Bookmark) {
		fmt.Print(bookmark.Frame(&b, cs))
		if i != lastIdx {
			fmt.Println()
		}
	})

	return nil
}

// JSON formats the bookmarks in JSON.
func JSON(bs *Slice) error {
	if bs.Empty() {
		slog.Debug("formatting config in JSON")
		fmt.Println(string(format.ToJSON(config.App)))
		return nil
	}

	slog.Debug("formatting bookmarks in JSON", "count", bs.Len())
	fmt.Println(string(format.ToJSON(bs.Items())))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs *Slice) error {
	s := config.App.Colorscheme
	cs, ok := color.Schemes[s]
	if !ok {
		return fmt.Errorf("%w: %q", color.ErrColorSchemeUnknown, s)
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	bs.ForEach(func(b Bookmark) {
		fmt.Print(bookmark.Oneline(&b, cs))
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
	slog.Info("selected field", "field", f)

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
