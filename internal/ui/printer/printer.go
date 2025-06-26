package printer

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
)

// Records prints the bookmarks in a frame format with the given colorscheme.
func Records(bs []*bookmark.Bookmark) error {
	lastIdx := len(bs) - 1
	for i := range bs {
		fmt.Print(bookmark.Frame(bs[i]))

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

	tags, err := db.TagsList(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(strings.Join(tags, "\n"))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs []*bookmark.Bookmark) error {
	for i := range bs {
		fmt.Print(bookmark.Oneline(bs[i]))
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
		fmt.Print(db.RepoSummaryFromPath(c, fname))
	}

	return nil
}

// RecordsJSON formats the bookmarks in RecordsJSON.
func RecordsJSON(bs []*bookmark.Bookmark) error {
	slog.Debug("formatting bookmarks in JSON", "count", len(bs))
	r := make([]*bookmark.BookmarkJSON, 0, len(bs))
	for _, b := range bs {
		r = append(r, b.ToJSON())
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

	tags, err := db.TagsCounter(r)
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
		fmt.Print(db.RepoSummaryFromPath(c, p+".enc"))
		return nil
	}

	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer r.Close()

	r.Cfg.BackupFiles, _ = r.ListBackups()
	if j {
		b, err := port.ToJSON(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(string(b))

		return nil
	}

	fmt.Print(db.Info(c, r))

	return nil
}
