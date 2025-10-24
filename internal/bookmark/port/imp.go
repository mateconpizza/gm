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

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// Browser imports bookmarks from a supported browser.
func Browser(a *app.Context) error {
	br, ok := getBrowser(selectBrowser(a.Console()))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}

	// find bookmarks
	bs, err := br.Import(a.Console(), a.Cfg.Flags.Yes)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(a, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
	}

	return IntoRepo(a, bs)
}

// Database imports bookmarks from a database.
func Database(a *app.Context, srcDB, destDB *db.SQLite) error {
	cfg := config.New()
	m := menu.New[bookmark.Bookmark](
		menu.WithConfig(a.Cfg.Menu),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import"),
		menu.WithPreview(cfg.Cmd+" -n "+srcDB.Name()+" records {1}"),
		menu.WithInterruptFn(func(err error) { // build interrupt cleanup
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	items, err := srcDB.All(a.Ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// BUG: fix!!!
	rec := make([]bookmark.Bookmark, 0, len(items))
	for i := range items {
		rec = append(rec, *items[i])
	}

	m.SetItems(rec)
	m.SetPreprocessor(func(b *bookmark.Bookmark) string {
		return txt.Oneline(a.Console(), b)
	})

	records, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(records))
	for i := range records {
		bookmarks = append(bookmarks, &records[i])
	}

	a.SetDatabase(destDB)

	c := a.Console()
	f := c.Frame()
	bookmarks = Deduplicate(a.Ctx, c, a.DB, bookmarks)
	n := len(bookmarks)
	if n == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	if err := a.DB.InsertMany(a.Ctx, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d record/s from %s", n, srcDB.Name())))

	return nil
}

// IntoRepo import records into the database.
func IntoRepo(a *app.Context, records []*bookmark.Bookmark) error {
	c := a.Console()
	n := len(records)
	if !a.Cfg.Flags.Force && n > 1 {
		if err := c.ConfirmErr(fmt.Sprintf("import %d records?", n), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()

	if err := a.DB.InsertMany(a.Ctx, records); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d record/s", n)))

	return nil
}

// FromBackup imports bookmarks from a backup.
func FromBackup(a *app.Context, destDB, srcDB *db.SQLite) error {
	c := a.Console()
	f, t, p := c.Frame(), c.Term(), c.Palette()
	f.Headerln(p.BrightYellow("Import bookmarks from backup: ") + p.GrayItalic(srcDB.Name())).Flush()

	m := menu.New[bookmark.Bookmark](
		menu.WithMultiSelection(),
		menu.WithConfig(a.Cfg.Menu),
		menu.WithPreview(fmt.Sprintf("%s -n ./backup/%s {1}", a.Cfg.Cmd, srcDB.Name())),
		menu.WithInterruptFn(t.InterruptFn),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'"),
	)

	defer t.CancelInterruptHandler()

	bookmarks, err := srcDB.All(a.Ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	rec := make([]bookmark.Bookmark, 0, len(bookmarks))
	for i := range bookmarks {
		rec = append(rec, *bookmarks[i])
	}

	m.SetItems(rec)
	m.SetPreprocessor(func(b *bookmark.Bookmark) string {
		return txt.Oneline(c, b)
	})

	items, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	result := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}

	// update which repo to insert
	a.SetDatabase(destDB)

	dRecords := Deduplicate(a.Ctx, c, a.DB, result)
	if len(dRecords) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	return IntoRepo(a, dRecords)
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
func Deduplicate(ctx context.Context, c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	const maxItemsToShow = 20
	originalLen := len(bs)
	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	discarted := make([]*bookmark.Bookmark, 0, len(bs))

	for _, b := range bs {
		if _, exists := r.Has(ctx, b.URL); exists {
			slog.Warn("deduplicate", "url", b.URL)
			discarted = append(discarted, b)
			continue
		}
		filtered = append(filtered, b)
	}

	p := c.Palette()
	n := len(filtered)
	if originalLen != n {
		s := fmt.Sprintf("%s %d/%d duplicate bookmarks", p.BrightYellow("skipping"), originalLen-n, originalLen)
		c.Warning(s + "\n").Flush()

		f := c.Frame()
		// show discarted bookmarks
		if len(discarted) <= maxItemsToShow && n != 0 {
			for _, b := range discarted {
				f.Midln(p.Italic(" " + txt.Shorten(b.URL, terminal.MinWidth)))
			}

			f.Flush()
		}
	}

	return filtered
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(a *app.Context, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	c := a.Console()
	f := c.Frame()
	bs = Deduplicate(a.Ctx, c, a.DB, bs)
	if len(bs) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return bs, nil
	}

	if !a.Cfg.Flags.Yes {
		q := fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs))
		if err := c.ConfirmErr(q, "y"); err != nil {
			if errors.Is(err, sys.ErrActionAborted) {
				return bs, nil
			}

			return nil, fmt.Errorf("%w", err)
		}
	}

	if err := metadata.ScrapeDescriptions(a.Ctx, bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}
