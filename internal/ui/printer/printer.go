// Package printer provides functions to format and print bookmark data,
// including records, tags, and repository information.
package printer

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// Records prints the bookmarks in a frame format with the given colorscheme.
func Records(bs []*bookmark.Bookmark) error {
	lastIdx := len(bs) - 1
	for i := range bs {
		fmt.Print(txt.Frame(bs[i]))

		if i != lastIdx {
			fmt.Println()
		}
	}

	return nil
}

// TagsList lists the tags.
func TagsList(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := db.TagsList(context.Background(), r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(strings.Join(tags, "\n"))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs []*bookmark.Bookmark) error {
	for i := range bs {
		fmt.Print(txt.Oneline(bs[i]))
	}

	return nil
}

func Notes(bs []*bookmark.Bookmark) error {
	printed := false
	for _, b := range bs {
		if b.Notes == "" {
			continue
		}
		if printed {
			fmt.Println()
		}
		fmt.Print(txt.Notes(b))
		printed = true
	}
	return nil
}

// ByField prints the selected field.
func ByField(bs []*bookmark.Bookmark, f string) error {
	printer := func(b *bookmark.Bookmark) error {
		f, err := b.Field(f)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(f)

		return nil
	}
	slog.Info("selected field", "field", f)

	for i := range bs {
		if err := printer(bs[i]); err != nil {
			return err
		}
	}

	return nil
}

// DatabasesList lists the available databases.
func DatabasesList(c *ui.Console, p string) error {
	fs, err := files.FindByExtList(p, ".db", ".enc")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(fs)
	if n > 1 {
		nColor := color.BrightCyan(n).Bold().String()
		c.F.Header(nColor + " database/s found\n").Row("\n").Flush()
	}

	for _, fname := range fs {
		fmt.Print(summary.RepoFromPath(c, fname))
	}

	return nil
}

// DatabasesTable shows a simple table in database information.
func DatabasesTable(p string) error {
	fs, err := files.FindByExtList(p, ".db", ".enc")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	headers := []string{"Name", "Bookmarks", "Tags", "Size", "Path"}
	rows := [][]string{}

	t := strconv.Itoa

	files.PrioritizeFile(fs, config.MainDBName)

	for _, fpath := range fs {
		ext := filepath.Ext(fpath)
		collapsePath := files.CollapseHomeDir(fpath)
		cleanName := files.StripSuffixes(filepath.Base(fpath))
		fsize := files.SizeFormatted(fpath)

		if ext == locker.Extension {
			cleanName = color.BrightMagenta(cleanName).String()
			cleanName += color.BrightGray(" (locked)").Italic().String()
			rows = append(rows, []string{cleanName, "-", "-", fsize, collapsePath})
			continue
		}

		s, err := dbtask.NewRepoStats(fpath)
		if err != nil {
			return err
		}

		if s.Name == config.MainDBName {
			cleanName = color.BrightYellow(cleanName).String()
			cleanName += color.BrightGray(" (default)").Italic().String()
		}

		rows = append(rows, []string{cleanName, t(s.Bookmarks), t(s.Tags), fsize, collapsePath})
	}

	fmt.Print(txt.CreateSimpleTable(headers, rows))

	return nil
}

// RecordsJSON formats the bookmarks in RecordsJSON.
func RecordsJSON(bs []*bookmark.Bookmark) error {
	slog.Debug("formatting bookmarks in JSON", "count", len(bs))
	r := make([]*bookmark.BookmarkJSON, 0, len(bs))
	for _, b := range bs {
		r = append(r, b.JSON())
	}

	j, err := port.ToJSON(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(string(j))

	return nil
}

// TagsJSON formats the tags counter in JSON.
func TagsJSON(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := r.TagsCounter(context.Background())
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	j, err := port.ToJSON(tags)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(string(j))

	return nil
}

// RepoInfo prints the database info.
func RepoInfo(c *ui.Console, p string, j bool) error {
	if err := locker.IsLocked(p); err != nil {
		fmt.Print(summary.RepoFromPath(c, p+".enc"))
		return nil
	}

	store, err := db.New(p)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer store.Close()

	// FIX: Implement ListBackups
	if j {
		b, err := port.ToJSON(store)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(string(b))

		return nil
	}

	r, err := db.New(store.Cfg.Fullpath())
	if err != nil {
		return err
	}

	info := summary.Info(c, r)

	g, err := git.Info(c, p)
	if err != nil {
		return fmt.Errorf("git: %w", err)
	}

	if g != "" {
		info += g
	}

	fmt.Print(info)

	return nil
}
