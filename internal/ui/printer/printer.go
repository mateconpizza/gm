// Package printer provides functions to format and print bookmark data,
// including records, tags, and repository information.
package printer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrUnknownFormat = errors.New("unknown format")
)

var ValidFormats = []string{
	"oneline",
	"json",
	"id",
	"url",
	"title",
	"tags",
	"desc",
	"notes",
}

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
func TagsList(ctx context.Context, p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := db.TagsList(ctx, r)
	if err != nil {
		return fmt.Errorf("tagslist: %w", err)
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

// Notes formats the bookmarks notes.
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

		if f == "" {
			return nil
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

// DatabasesTable shows a simple table in database information.
func DatabasesTable(ctx context.Context, p string) error {
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

		s, err := dbtask.NewRepoStats(ctx, fpath)
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
func TagsJSON(ctx context.Context, p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := r.TagsCounter(ctx)
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
func RepoInfo(a *app.Context) error {
	// FIX: Test RepoInfo()
	if err := locker.IsLocked(a.Cfg.DBPath); err != nil {
		fmt.Print(summary.RepoFromPath(a, a.Cfg.DBPath+".enc", a.Cfg.Path.Backup))
		return nil
	}

	// FIX: Implement ListBackups
	if a.Cfg.Flags.JSON {
		b, err := port.ToJSON(a.DB)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(string(b))

		return nil
	}

	info := summary.Info(a)

	g, err := git.Info(a.Console, a.Cfg.DBPath, a.Cfg.Git)
	if err != nil {
		return fmt.Errorf("git: %w", err)
	}

	if g != "" {
		info += g
	}

	fmt.Print(info)

	return nil
}

func parseDisplayFormat(format string) (formatType, field string, err error) {
	switch format {
	case "o", "oneline", "one":
		return "oneline", "", nil
	case "j", "json":
		return "json", "", nil
	case "id", "i", "1", "url", "u", "2", "title", "t", "3",
		"tags", "T", "4", "desc", "d", "5", "notes", "n", "6":
		return "field", format, nil
	default:
		return "", "", fmt.Errorf(
			"%w: %q (use: %s)",
			ErrInvalidFormat,
			format,
			strings.Join(ValidFormats, "|"),
		)
	}
}

func Display(format string, bs []*bookmark.Bookmark) error {
	formatType, field, err := parseDisplayFormat(format)
	if err != nil {
		return err
	}

	switch formatType {
	case "oneline":
		return Oneline(bs)
	case "json":
		return RecordsJSON(bs)
	case "field":
		return ByField(bs, field)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownFormat, formatType)
	}
}
