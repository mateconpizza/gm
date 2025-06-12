package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
)

type colorSchemes = map[string]*color.Scheme

// Print prints the bookmarks in a frame format with the given colorscheme.
func Print(bs *slice.Slice[bookmark.Bookmark]) error {
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		return err
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	lastIdx := bs.Len() - 1
	bs.ForEachIdx(func(i int, b bookmark.Bookmark) {
		fmt.Print(bookmark.Frame(&b, cs))
		if i != lastIdx {
			fmt.Println()
		}
	})

	return nil
}

// JSONSlice formats the bookmarks in JSONSlice.
func JSONSlice(bs *slice.Slice[bookmark.Bookmark]) error {
	slog.Debug("formatting bookmarks in JSON", "count", bs.Len())
	var r []*bookmark.BookmarkJSON
	bs.ForEach(func(b bookmark.Bookmark) {
		r = append(r, b.ToJSON())
	})
	j, err := format.ToJSON(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(string(j))

	return nil
}

// JSONTags formats the tags counter in JSON.
func JSONTags(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()
	tags, err := db.TagsCounter(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	j, err := format.ToJSON(tags)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(string(j))

	return nil
}

// ListTags lists the tags.
func ListTags(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := db.TagsList(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(strings.Join(tags, "\n"))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs *slice.Slice[bookmark.Bookmark]) error {
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		return err
	}
	cs.Enabled = config.App.Color
	slog.Info("colorscheme loaded", "name", cs.Name)

	bs.ForEach(func(b bookmark.Bookmark) {
		fmt.Print(bookmark.Oneline(&b, cs))
	})

	return nil
}

// ByField prints the selected field.
func ByField(bs *slice.Slice[bookmark.Bookmark], f string) error {
	printer := func(b bookmark.Bookmark) error {
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

	return cs, nil
}

// fzfFormatter returns a function to format a bookmark for the FZF menu.
func fzfFormatter(m bool) func(b *bookmark.Bookmark) string {
	cs, err := getColorScheme(config.App.Colorscheme)
	if err != nil {
		slog.Error("getting colorscheme", slog.String("error", err.Error()))
	}
	slog.Info("colorscheme loaded", "name", cs.Name)

	switch {
	case m:
		return func(b *bookmark.Bookmark) string {
			return bookmark.Multiline(b, cs)
		}
	default:
		return func(b *bookmark.Bookmark) string {
			return bookmark.Oneline(b, cs)
		}
	}
}

// ListDatabases lists the available databases.
func ListDatabases(p string) error {
	fs, err := files.FindByExtList(p, ".db", ".enc")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	n := len(fs)
	if n > 1 {
		nColor := color.BrightCyan(n).Bold().String()
		f.Header(nColor + " database/s found\n").Row("\n").Flush()
	}

	for _, f := range fs {
		fmt.Print(db.RepoSummaryFromPath(f))
	}

	return nil
}

// RepoInfo prints the database info.
func RepoInfo(p string, j bool) error {
	if err := locker.IsLocked(p); err != nil {
		fmt.Print(db.RepoSummaryFromPath(p + ".enc"))
		return nil
	}
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer r.Close()
	r.Cfg.BackupFiles, _ = r.ListBackups()
	if j {
		b, err := format.ToJSON(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		fmt.Println(string(b))

		return nil
	}

	fmt.Print(db.Info(r))

	return nil
}

// MenuForRecords returns a FZF menu for showing records.
func MenuForRecords[T comparable](cmd *cobra.Command) *menu.Menu[T] {
	mo := []menu.OptFn{
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(config.App.Cmd + " --name " + config.App.DBName + " records {1}"),
		menu.WithKeybinds(
			config.FzfKeybindEdit(),
			config.FzfKeybindOpen(),
			config.FzfKeybindQR(),
			config.FzfKeybindOpenQR(),
			config.FzfKeybindYank(),
		),
	}
	multi, err := cmd.Flags().GetBool("multiline")
	if err != nil {
		slog.Debug("getting 'Multiline' flag", "error", err.Error())
		multi = false
	}
	if multi {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
