// Package port provides functionalities for importing and exporting data,
// supporting various sources and formats including browsers, databases, Git
// repositories, JSON, and GPG encrypted files.
package port

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
func Database(ctx context.Context, d *deps.Deps, srcDB *db.SQLite) error {
	destDB, err := d.Repository()
	if err != nil {
		return err
	}

	app, err := d.Application(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	m := picker.New[*bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import"),
		menu.WithMultiSelection(),
		menu.WithPreview(menu.PreviewCmd(app.Command(), srcDB.Name(), "{1}")),
		menu.WithInterruptFn(func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	bs, err := srcDB.All(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := d.Console()
	m.SetFormatter(func(b **bookmark.Bookmark) string { return formatter.OnelineFunc(c, *b) })

	bs, err = m.Select(bs)
	if err != nil {
		return err
	}

	bs, err = DeduplicateReport(ctx, c, destDB, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		p := c.Palette()
		c.Frame().Error("no new bookmark found, ").
			Textln(p.BrightYellow.Wrap("skipping import", p.Italic)).
			Flush()
		return sys.ErrExitFailure
	}

	if err := destDB.InsertMany(ctx, bs); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("imported ", len(bs), " record/s from ", srcDB.Name()))
}

// FromBackup imports bookmarks from a backup.
func FromBackup(ctx context.Context, d *deps.Deps, destDB, srcDB *db.SQLite) error {
	c := d.Console()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	m := picker.New[*bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'"),
		menu.WithInterruptFn(c.Term().InterruptFn()),
		menu.WithMultiSelection(),
		menu.WithPreview(menu.PreviewCmd(app.Command(), "./backup/"+srcDB.Name(), "{+1}")),
	)

	defer c.Term().CancelInterruptHandler()

	bookmarks, err := srcDB.All(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	m.SetFormatter(func(b **bookmark.Bookmark) string {
		return formatter.OnelineFunc(c, *b)
	})

	bookmarks, err = m.Select(bookmarks)
	if err != nil {
		return err
	}

	// update which repo to insert
	d.SetRepo(destDB)

	r, err := d.Repository()
	if err != nil {
		return err
	}

	result, err := DeduplicateReport(ctx, c, r, bookmarks)
	if err != nil {
		return err
	}

	if len(result) == 0 {
		p := c.Palette()
		c.Frame().Error("no new bookmark found, ").
			Textln(p.BrightYellow.Wrap("skipping import", p.Italic)).
			Flush()
		return sys.ErrExitFailure
	}

	return importPipeline(ctx, d, "from backup", srcDB.Name(), result)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("to JSON: %w", err)
	}

	return jsonData, nil
}

// DeduplicateReport removes duplicate bookmarks and reports skipped entries to the console.
func DeduplicateReport(ctx context.Context, c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	const maxItemsToShow = 10

	existing, err := r.All(ctx)
	if err != nil {
		return nil, err
	}

	fresh, duplicates := bookmark.Deduplicate(bs, existing)
	if len(duplicates) == 0 {
		return fresh, nil
	}

	p := c.Palette()
	skip := p.BrightYellow.Sprint("skipping")
	c.Warning(fmt.Sprintf("%s %d/%d duplicate bookmarks\n", skip, len(duplicates), len(bs))).
		Flush()

	f := c.Frame()

	for i, b := range duplicates {
		if i >= maxItemsToShow {
			f.Midln(p.Dim.With(p.Italic).Sprintf(" ... and %d more", len(duplicates)-i))
			break
		}

		f.Midln(p.Dim.Wrap(" "+txt.Shorten(b.URL, c.MinWidth()), p.Italic))
	}

	f.Rowln().
		Flush()

	return fresh, nil
}
