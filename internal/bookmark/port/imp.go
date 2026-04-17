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
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
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
func Browser(d *deps.Deps) error {
	br, ok := getBrowser(selectBrowser(d.Console()))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}

	// find bookmarks
	bs, err := br.Import(d.Console(), d.App.Flags.Yes)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(d, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
	}

	return IntoRepo(d, bs)
}

// Database imports bookmarks from a database.
func Database(d *deps.Deps, srcDB, destDB *db.SQLite) error {
	app, err := d.Application()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithOutputColor(app.Flags.Color),
		menu.WithConfig(d.App.Menu),
		menu.WithHeader("select record/s to import"),
		menu.WithMultiSelection(),
		menu.WithPreview(app.PreviewCmd(srcDB.Name())+" {1}"),
		menu.WithInterruptFn(func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	items, err := srcDB.All(d.Context())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// BUG: fix!!!
	rec := make([]bookmark.Bookmark, 0, len(items))
	for i := range items {
		rec = append(rec, *items[i])
	}

	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return txt.Oneline(d.Console(), b)
	})

	records, err := m.Select(rec)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(records))
	for i := range records {
		bookmarks = append(bookmarks, &records[i])
	}

	d.SetRepo(destDB)

	c := d.Console()
	f := c.Frame()
	bookmarks = Deduplicate(d.Context(), c, d.Repo, bookmarks)
	n := len(bookmarks)
	if n == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	if err := d.Repo.InsertMany(d.Context(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d record/s from %s", n, srcDB.Name())))

	return nil
}

// IntoRepo import records into the database.
func IntoRepo(d *deps.Deps, records []*bookmark.Bookmark) error {
	c := d.Console()
	n := len(records)
	if !d.App.Flags.Force && n > 1 {
		if err := c.ConfirmErr(fmt.Sprintf("import %d records?", n), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()

	if err := d.Repo.InsertMany(d.Context(), records); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d record/s", n)))

	return nil
}

// FromBackup imports bookmarks from a backup.
func FromBackup(d *deps.Deps, destDB, srcDB *db.SQLite) error {
	c := d.Console()
	f, t, p := c.Frame(), c.Term(), c.Palette()

	m := menu.New[bookmark.Bookmark](
		menu.WithOutputColor(d.App.Flags.Color),
		menu.WithConfig(d.App.Menu),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'"),
		menu.WithInterruptFn(t.InterruptFn),
		menu.WithMultiSelection(),
		menu.WithPreview(d.App.PreviewCmd("./backup/"+srcDB.Name())+" {+1}"),
	)

	defer t.CancelInterruptHandler()

	bookmarks, err := srcDB.All(d.Context())
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	rec := make([]bookmark.Bookmark, 0, len(bookmarks))
	for i := range bookmarks {
		rec = append(rec, *bookmarks[i])
	}

	m.SetFormatter(func(b *bookmark.Bookmark) string { return txt.Oneline(c, b) })
	items, err := m.Select(rec)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	h := p.BrightYellow.Sprint("Import bookmarks from backup: ")
	f.Headerln(h + p.BrightBlack.Wrap(srcDB.Name(), p.Italic)).Flush()

	result := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}

	// update which repo to insert
	d.SetRepo(destDB)

	dRecords := Deduplicate(d.Context(), c, d.Repo, result)
	if len(dRecords) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	return IntoRepo(d, dRecords)
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
		skip := p.BrightYellow.Sprint("skipping")
		s := fmt.Sprintf("%s %d/%d duplicate bookmarks", skip, originalLen-n, originalLen)
		c.Warning(s + "\n").Flush()

		f := c.Frame()
		// show discarted bookmarks
		if len(discarted) <= maxItemsToShow && n != 0 {
			for _, b := range discarted {
				f.Midln(p.Italic.Sprint(" " + txt.Shorten(b.URL, terminal.MinWidth)))
			}

			f.Flush()
		}
	}

	return filtered
}

// DeduplicateByURL removes bookmarks from `incoming` that already exist in `existing`.
// It compares bookmarks by their URL and returns a new slice with unique entries.
func DeduplicateByURL(existing, incoming []*bookmark.Bookmark) []*bookmark.Bookmark {
	// FIX: replace `Deduplicate` with this
	if len(incoming) == 0 {
		return nil
	}
	if len(existing) == 0 {
		return incoming
	}

	// Fast lookup map of existing URLs.
	seen := make(map[string]struct{}, len(existing))
	for _, b := range existing {
		if b.URL != "" {
			seen[strings.TrimSuffix(b.URL, "/")] = struct{}{}
		}
	}

	// Filter incoming bookmarks.
	filtered := make([]*bookmark.Bookmark, 0, len(incoming))
	for _, b := range incoming {
		if b == nil || b.URL == "" {
			continue
		}
		if _, dup := seen[b.URL]; dup {
			continue
		}
		seen[b.URL] = struct{}{}
		filtered = append(filtered, b)
	}

	return filtered
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	c := d.Console()
	f := c.Frame()
	bs = Deduplicate(d.Context(), c, d.Repo, bs)
	if len(bs) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return bs, nil
	}

	if !d.App.Flags.Yes {
		q := fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs))
		if err := c.ConfirmErr(q, "y"); err != nil {
			if errors.Is(err, sys.ErrActionAborted) {
				return bs, nil
			}

			return nil, fmt.Errorf("%w", err)
		}
	}

	if err := metadata.ScrapeDescriptions(d.Context(), bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}
