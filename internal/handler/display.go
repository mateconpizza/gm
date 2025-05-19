package handler

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/sys/files"
)

type colorSchemes = map[string]*color.Scheme

// Print prints the bookmarks in a frame format with the given colorscheme.
func Print(bs *Slice) error {
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		return err
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	lastIdx := bs.Len() - 1
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
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		return err
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

// LoadColorSchemesFiles loads available colorschemes.
func LoadColorSchemesFiles(p string, schemes colorSchemes) (colorSchemes, error) {
	if !files.Exists(p) {
		slog.Warn("load colorscheme", "path not found", p)
		return schemes, color.ErrColorSchemePath
	}
	fs, err := files.FindByExtList(p, "yaml")
	if err != nil {
		return schemes, fmt.Errorf("%w", err)
	}

	if len(fs) == 0 {
		return schemes, nil
	}

	for _, s := range fs {
		var cs *color.Scheme
		if err := files.YamlRead(s, &cs); err != nil {
			return schemes, fmt.Errorf("%w", err)
		}
		if err := cs.Validate(); err != nil {
			return schemes, fmt.Errorf("%w", err)
		}

		schemes[cs.Name] = cs
	}

	return schemes, nil
}

// getColorScheme returns a colorscheme based on the given name.
//
// If the colorscheme is not found, the default colorscheme is returned.
func getColorScheme(s string) (*color.Scheme, error) {
	schemes, err := LoadColorSchemesFiles(config.App.Path.Colorschemes, color.DefaultSchemes)
	if err != nil && !errors.Is(err, color.ErrColorSchemePath) {
		return nil, fmt.Errorf("%w", err)
	}
	color.DefaultSchemes = schemes

	cs, ok := color.DefaultSchemes[s]
	if !ok {
		slog.Warn("printing bookmarks", "error", s+" not found, using default")
		cs = color.DefaultSchemes["default"]
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	return cs, nil
}

// FzfFormatter returns a function to format a bookmark for the FZF menu.
func FzfFormatter(m bool, colorScheme string) func(b *Bookmark) string {
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		slog.Error("getting colorscheme", slog.String("error", err.Error()))
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	switch {
	case m:
		return func(b *Bookmark) string {
			return bookmark.Multiline(b, cs)
		}
	default:
		return func(b *Bookmark) string {
			return bookmark.Oneline(b, cs)
		}
	}
}
