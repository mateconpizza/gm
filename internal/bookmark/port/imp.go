// Package port provides functionalities for importing and exporting data,
// supporting various sources and formats including browsers, databases, Git
// repositories, JSON, and GPG encrypted files.
package port

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var ErrNothingToImport = errors.New("nothing to import")

type selector[T any] interface {
	Select(items []T) ([]T, error)
}

type storeReader interface {
	Name() string
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
	Close()
}

// Database imports bookmarks from a database.
func Database(ctx context.Context, d *deps.Deps, srcDB storeReader) error {
	app, err := d.Application(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	bs, err := srcDB.All(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := d.Console()
	fm := app.Formatter()
	p := fm.Menu.Placeholder()

	destDB, err := d.Repository()
	if err != nil {
		return err
	}

	m := picker.New[*bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import"),
		menu.WithMultiSelection(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), srcDB.Name(), p.Single())),
		menu.WithInterruptFn(func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)
	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(c, b)
	})

	bs, err = m.Select(bs)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from database", srcDB.Name(), bs)
}

func ImportFromDatabase(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	srcPath, err := dbops.Select(ctx, d, app.Path.DB())
	if err != nil {
		return err
	}

	rSrc, err := db.New(ctx, srcPath)
	if err != nil {
		return err
	}
	defer rSrc.Close()

	return Database(ctx, d, rSrc)
}

func ImportFromBackup(ctx context.Context, d *deps.Deps) error {
	backups, err := dbops.Backups(ctx, d)
	if err != nil {
		return err
	}
	backupPath, err := dbops.SelectBackup(ctx, d, backups)
	if err != nil {
		return err
	}
	srcRepo, err := db.New(ctx, backupPath)
	if err != nil {
		return err
	}
	defer srcRepo.Close()

	c := d.Console()
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	fm := formatter.Default()
	p := fm.Menu.Placeholder()
	m := picker.New[*bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import from '"+srcRepo.Name()+"'"),
		menu.WithInterruptFn(c.Term().InterruptFn()),
		menu.WithMultiSelection(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), "./backup/"+srcRepo.Name(), p.Single())),
	)

	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(c, b)
	})

	return importBookmarksFromBackup(ctx, d, srcRepo, m)
}

// importBookmarksFromBackup imports bookmarks from a backup.
func importBookmarksFromBackup(ctx context.Context, d *deps.Deps, srcDB storeReader, m selector[*bookmark.Bookmark]) error {
	bookmarks, err := srcDB.All(ctx)
	if err != nil {
		return err
	}

	c := d.Console()
	defer c.Term().CancelInterruptHandler()

	bookmarks, err = m.Select(bookmarks)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from backup", srcDB.Name(), bookmarks)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("to JSON: %w", err)
	}

	return jsonData, nil
}

// DeduplicateReport removes duplicate bookmarks and reports skipped entries to
// the console.
func DeduplicateReport(ctx context.Context, c *ui.Console, r storeReader, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
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
