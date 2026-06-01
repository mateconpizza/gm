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

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var ErrNothingToImport = errors.New("nothing to import")

// Database imports bookmarks from a database.
func Database(ctx context.Context, d *deps.Deps, srcDB, destDB *db.SQLite) error {
	app, err := d.Application(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	m := picker.New[bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import"),
		menu.WithMultiSelection(),
		menu.WithPreview(app.PreviewCmd(srcDB.Name(), "{1}")),
		menu.WithInterruptFn(func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	items, err := srcDB.All(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	rec := make([]bookmark.Bookmark, 0, len(items))
	for i := range items {
		rec = append(rec, *items[i])
	}

	m.SetFormatter(func(b *bookmark.Bookmark) string { return formatter.OnelineFunc(d.Console(), b) })
	records, err := m.Select(rec)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(records))
	for i := range records {
		bookmarks = append(bookmarks, &records[i])
	}

	d.SetRepo(destDB)

	c, f := d.Console(), d.Console().Frame()
	r, err := d.Repository()
	if err != nil {
		return err
	}
	bookmarks, err = DeduplicateReport(ctx, c, r, bookmarks)
	if err != nil {
		return err
	}
	n := len(bookmarks)
	if n == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	if err := r.InsertMany(ctx, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return c.Print(ctx, c.SuccessMesg("imported ", n, " record/s from ", srcDB.Name()))
}

// IntoRepo import records into the database.
func IntoRepo(ctx context.Context, d *deps.Deps, records []*bookmark.Bookmark) error {
	c := d.Console()
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if !app.Flags.Force && len(records) > 1 {
		records, err = promptImportSelection(ctx, d.Console(), app, records)
		if err != nil {
			return err
		}
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}

	if err := r.InsertMany(ctx, records); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("imported %d record/s\n", len(records))))
}

// FromBackup imports bookmarks from a backup.
func FromBackup(ctx context.Context, d *deps.Deps, destDB, srcDB *db.SQLite) error {
	c := d.Console()
	f, t, p := c.Frame(), c.Term(), c.Palette()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	m := picker.New[bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'"),
		menu.WithInterruptFn(t.InterruptFn()),
		menu.WithMultiSelection(),
		menu.WithPreview(app.PreviewCmd("./backup/"+srcDB.Name(), "{+1}")),
	)

	defer t.CancelInterruptHandler()

	bookmarks, err := srcDB.All(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	rec := make([]bookmark.Bookmark, 0, len(bookmarks))
	for i := range bookmarks {
		rec = append(rec, *bookmarks[i])
	}

	m.SetFormatter(func(b *bookmark.Bookmark) string { return formatter.OnelineFunc(c, b) })
	items, err := m.Select(rec)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	h := p.BrightYellow.Sprint("Import bookmarks from backup: ")
	f.Headerln(h + p.Gray.Wrap(srcDB.Name(), p.Italic)).Flush()

	result := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}

	// update which repo to insert
	d.SetRepo(destDB)

	r, err := d.Repository()
	if err != nil {
		return err
	}
	dRecords, err := DeduplicateReport(ctx, c, r, result)
	if err != nil {
		return err
	}

	if len(dRecords) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	return IntoRepo(ctx, d, dRecords)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return jsonData, nil
}

// Deduplicate partitions bs into fresh and duplicate bookmarks.
func Deduplicate(bs, existing []*bookmark.Bookmark) (fresh, duplicates []*bookmark.Bookmark) {
	fresh = make([]*bookmark.Bookmark, 0, len(bs))
	duplicates = make([]*bookmark.Bookmark, 0, len(bs))
	seen := make(map[string]struct{}, len(existing))

	for _, b := range existing {
		if b == nil {
			continue
		}
		seen[b.URL] = struct{}{}
	}

	for _, b := range bs {
		if b == nil {
			continue
		}

		if _, ok := seen[b.URL]; ok {
			slog.Warn("deduplicate", "url", b.URL)
			duplicates = append(duplicates, b)
			continue
		}

		fresh = append(fresh, b)
	}

	return fresh, duplicates
}

// DeduplicateReport removes duplicate bookmarks and reports skipped entries to the console.
func DeduplicateReport(
	ctx context.Context,
	c *ui.Console,
	r *db.SQLite,
	bs []*bookmark.Bookmark,
) ([]*bookmark.Bookmark, error) {
	const maxItemsToShow = 10

	existing, err := r.All(ctx)
	if err != nil {
		return nil, err
	}

	fresh, duplicates := Deduplicate(bs, existing)
	if len(duplicates) == 0 {
		return fresh, nil
	}

	p := c.Palette()
	skip := p.BrightYellow.Sprint("skipping")
	s := fmt.Sprintf("%s %d/%d duplicate bookmarks", skip, len(duplicates), len(bs))
	c.Warning(s + "\n").Flush()

	f := c.Frame()
	for i, b := range duplicates {
		if i >= maxItemsToShow {
			f.Midln(p.Dim.With(p.Italic).Sprintf(" ... and %d more", len(duplicates)-i))
			break
		}
		f.Midln(p.Dim.Wrap(" "+txt.Shorten(b.URL, c.MinWidth()), p.Italic))
	}
	f.Rowln().Flush()

	return fresh, nil
}
