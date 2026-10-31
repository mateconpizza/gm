// Package port provides functionalities for importing and exporting data,
// supporting various sources and formats including browsers, databases, Git
// repositories, JSON, and GPG encrypted files.
package port

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"
	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var ErrNothingToImport = errors.New("nothing to import")

type selector[T any] interface {
	Select(ctx context.Context, items []T) ([]T, error)
}

type storeReader interface {
	Name() string
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
	Close()
}

func ImportFromDatabase(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	selected, err := dbops.NewDatabaseSelector(app).
		WithExclutions(app.Path.DB()).
		WithOpts(menu.WithHeader("choose a database to import from")).
		Select(ctx)
	if err != nil {
		return err
	}

	rSrc, err := db.New(ctx, selected[0])
	if err != nil {
		return err
	}
	defer rSrc.Close()

	return database(ctx, d, rSrc)
}

func ImportFromBackup(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	selected, err := dbops.NewBackupSelector(app).
		WithOpts(menu.WithHeader("choose a backup to import from")).
		Select(ctx)
	if err != nil {
		return err
	}

	backupPath := selected[0]

	srcRepo, err := db.New(ctx, backupPath)
	if err != nil {
		return err
	}
	defer srcRepo.Close()

	c := d.Console()

	fm := formatter.Default()
	p := fm.Menu.Placeholder()
	m := picker.New[*bookmark.Bookmark](
		app,
		menu.WithHeader("select record/s to import from '"+srcRepo.Name()+"'"),
		menu.WithInterruptFn(c.Term().InterruptFn()),
		menu.WithMultiSelection(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), "./backup/"+srcRepo.Name(), p.Single())),
		menu.WithHeaderKeymaps(),
		menu.WithKeybinds(menu.KeymapToggleAll()),
	)

	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(c, b)
	})

	return importBookmarksFromBackup(ctx, d, srcRepo, m)
}

// ImportFromDatabasePath import bookmarks from database.
func ImportFromDatabasePath(ctx context.Context, d *deps.Deps, path string) error {
	if err := files.ExistsErr(path); err != nil {
		return err
	}

	r, err := db.New(ctx, path)
	if err != nil {
		return err
	}
	defer r.Close()

	bs, err := r.All(ctx)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from database file", path, bs)
}

// ImportFromHTML import bookmarks from HTML Netscape file.
func ImportFromHTML(ctx context.Context, d *deps.Deps, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := bookio.ReadNetscapeHTML(f)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from HTML", path, bs)
}

func ImportFromJSON(ctx context.Context, d *deps.Deps, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := bookio.ReadJSON(f)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from JSON", path, bs)
}

func ImportFromCSV(ctx context.Context, d *deps.Deps, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := bookio.ReadCSV(f)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from CSV", path, bs)
}

func WriteYAML(w io.Writer, bs []*bookmark.Bookmark) error {
	recs := make([]*bookmark.Record, len(bs))
	for i, b := range bs {
		recs[i] = b.Record()
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)

	if err := enc.Encode(recs); err != nil {
		return err
	}

	return enc.Close() // flushes the encoder
}

// importPipeline handles deduplication, user prompting, and persistence.
func importPipeline(ctx context.Context, d *deps.Deps, source, from string, bs []*bookmark.Bookmark) error {
	c := d.Console()

	r, err := d.Repository()
	if err != nil {
		return err
	}

	printImportHeader(c, source, r.Fullpath(), from, len(bs))

	deduplicated, err := DeduplicateReport(ctx, c, r, bs)
	if err != nil {
		return err
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if len(deduplicated) == 0 {
		c.Frame().Error(ErrNothingToImport.Error() + "\n").Flush()
		return app.Failure()
	}

	if !app.Flags.Force && !app.Flags.Yes {
		deduplicated, err = promptImportSelection(ctx, d, deduplicated)
		if err != nil {
			return err
		}
	}

	if err := r.InsertMany(ctx, deduplicated); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("imported ", len(deduplicated), " bookmarks\n"))
}

func printImportHeader(c *ui.Console, header, fromName, toName string, n int) {
	p := c.Palette()
	bs := p.BrightGreen.With(p.Bold).Sprint("found") +
		p.Italic.Sprintf(" %d bookmarks found\n", n)

	value := func(s string) string {
		return p.BrightYellow.With(p.Italic).Sprint(
			files.CollapseHomeDir(s),
		)
	}

	c.NewBannerBuilder().
		WithTitle("Import Bookmarks " + header).
		WithTitleColor(p.BrightGreen.With(p.Bold).Sprint).
		WithTitleGlyph(c.Glyphs().Add).
		WithSubtitle("merge bookmarks into your collection").
		Build().
		Rowln().
		Info(txt.PaddedLine("source:", value(toName+"\n"))).
		Info(txt.PaddedLine("destination:", value(fromName+"\n"))).
		Rowln().
		Info(bs).
		Flush()
}

// promptImportSelection runs the interactive action loop.
func promptImportSelection(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	c := d.Console()

	for {
		n := len(bs)
		options := []string{"yes", "no"}
		if n > 1 {
			options = append(options, "select")
		}

		opt, err := c.Choose(
			ctx,
			fmt.Sprintf("import %d bookmarks into %q?", n, app.DBBaseName()),
			options,
			"y",
		)
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(opt) {
		case "no":
			return nil, app.Abort()

		case "select":
			fm := app.Formatter()
			fm.Menu.Opts = append(
				fm.Menu.Opts,
				menu.WithHeader("select record/s to import"),
				menu.WithInterruptFn(c.Term().InterruptFn()),
				menu.WithMultiSelection(),
				menu.WithKeybinds(menu.KeymapToggleAll()),
				menu.WithHeaderKeymaps(),
			)

			m := picker.New[*bookmark.Bookmark](app, fm.Menu.Opts...)
			m.SetFormatter(func(b *bookmark.Bookmark) string {
				return fm.Render(c, b)
			})

			bs, err = m.Select(ctx, bs)
			if err != nil {
				return nil, err
			}

		case "yes":
			return bs, nil
		}
	}
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

// database imports bookmarks from a database.
func database(ctx context.Context, d *deps.Deps, srcDB storeReader) error {
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
			app.Exit(err)
		}),
	)
	m.SetFormatter(func(b *bookmark.Bookmark) string {
		return fm.Render(c, b)
	})

	bs, err = m.Select(ctx, bs)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from database", srcDB.Name(), bs)
}

// importBookmarksFromBackup imports bookmarks from a backup.
func importBookmarksFromBackup(ctx context.Context, d *deps.Deps, srcDB storeReader, m selector[*bookmark.Bookmark]) error {
	bookmarks, err := srcDB.All(ctx)
	if err != nil {
		return err
	}

	c := d.Console()
	defer c.Term().CancelInterruptHandler()

	bookmarks, err = m.Select(ctx, bookmarks)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from backup", srcDB.Name(), bookmarks)
}
