// Package port provides functionalities for importing and exporting data,
// supporting various sources and formats including browsers, databases, Git
// repositories, JSON, and GPG encrypted files.
package port

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// Browser imports bookmarks from a supported browser.
func Browser(c *ui.Console, r *db.SQLite) error {
	br, ok := getBrowser(selectBrowser(c))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}
	// find bookmarks
	bs, err := br.Import(c, config.App.Flags.Force)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(c, r, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
	}

	return IntoRepo(c, r, bs)
}

// Database imports bookmarks from a database.
func Database(c *ui.Console, srcDB, destDB *db.SQLite) error {
	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
		menu.WithPreview(config.App.Cmd+" -n "+srcDB.Name()+" records {1}"),
		menu.WithInterruptFn(func(err error) { // build interrupt cleanup
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	items, err := srcDB.All(context.Background())
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// BUG: fix!!!
	rec := make([]bookmark.Bookmark, 0, len(items))
	for i := range items {
		rec = append(rec, *items[i])
	}

	m.SetItems(rec)
	m.SetPreprocessor(txt.Oneline)

	records, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(records))
	for i := range records {
		bookmarks = append(bookmarks, &records[i])
	}

	bookmarks = Deduplicate(c, destDB, bookmarks)
	n := len(bookmarks)
	if n == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	if err := destDB.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("imported %d record/s from %s\n", n, srcDB.Name())))

	return nil
}

// IntoRepo import records into the database.
func IntoRepo(c *ui.Console, r *db.SQLite, records []*bookmark.Bookmark) error {
	n := len(records)
	if !config.App.Flags.Force && n > 1 {
		if err := c.ConfirmErr(fmt.Sprintf("import %d records?", n), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()

	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	fmt.Print(c.SuccessMesg(fmt.Sprintf("imported %d record/s\n", n)))

	return nil
}

// FromBackup imports bookmarks from a backup.
func FromBackup(c *ui.Console, destDB, srcDB *db.SQLite) error {
	s := color.BrightYellow("Import bookmarks from backup: ").String()
	c.F.Headerln(s + color.Gray(srcDB.Name()).Italic().String()).Flush()
	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(fmt.Sprintf("%s -n ./backup/%s {1}", config.App.Cmd, srcDB.Name())),
		menu.WithInterruptFn(c.T.InterruptFn),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'", false),
	)

	defer c.T.CancelInterruptHandler()

	bookmarks, err := srcDB.All(context.Background())
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	rec := make([]bookmark.Bookmark, 0, len(bookmarks))
	for i := range bookmarks {
		rec = append(rec, *bookmarks[i])
	}

	m.SetItems(rec)
	m.SetPreprocessor(txt.Oneline)

	items, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	result := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}

	dRecords := Deduplicate(c, destDB, result)
	if len(dRecords) == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	return IntoRepo(c, destDB, dRecords)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return jsonData, nil
}

// Deduplicate removes duplicate bookmarks.
func Deduplicate(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	originalLen := len(bs)
	filtered := make([]*bookmark.Bookmark, 0, len(bs))

	for _, b := range bs {
		if _, exists := r.Has(context.Background(), b.URL); exists {
			slog.Warn("deduplicate", "url", b.URL)
			continue
		}
		filtered = append(filtered, b)
	}

	n := len(filtered)
	if originalLen != n {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-n)
		c.Warning(s + "\n").Flush()
	}

	return filtered
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(
	c *ui.Console,
	r *db.SQLite,
	bs []*bookmark.Bookmark,
) ([]*bookmark.Bookmark, error) {
	bs = Deduplicate(c, r, bs)
	if len(bs) == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return bs, nil
	}

	if !config.App.Flags.Force {
		if err := c.ConfirmErr(fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs)), "y"); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				return bs, nil
			}

			return nil, fmt.Errorf("%w", err)
		}
	}

	if err := parser.ScrapeMissingDescription(bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}
