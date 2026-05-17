package port

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// FromFile import bookmarks from database.
func FromFile(d *deps.Deps, f string) error {
	bs, err := bookmarksFromFile(d.Context(), f)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	return importPipeline(d, r, "from file", f, bs)
}

// FromHTML import bookmarks from HTML Netscape file.
func FromHTML(d *deps.Deps, f string) error {
	bs, err := bookmarksFromHTML(f)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	return importPipeline(d, r, "from HTML", f, bs)
}

// importPipeline handles deduplication, user prompting, and persistence.
func importPipeline(d *deps.Deps, r *db.SQLite, source, from string, bs []*bookmark.Bookmark) error {
	ctx, c := d.Context(), d.Console()

	printImportHeader(c, source, files.StripSuffixes(r.Name()), from, len(bs))

	deduplicated, err := DeduplicateReport(ctx, c, r, bs)
	if err != nil {
		return err
	}

	if len(deduplicated) == 0 {
		c.Frame().Error(ErrNothingToImport.Error() + "\n").Flush()
		return sys.ErrExitFailure
	}

	app, err := d.Application()
	if err != nil {
		return err
	}

	if !app.Flags.Force && !app.Flags.Yes {
		deduplicated, err = promptImportSelection(c, app, deduplicated)
		if err != nil {
			return err
		}
	}

	if err := r.InsertMany(ctx, deduplicated); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("imported ", len(deduplicated), " bookmarks\n"))
}

// bookmarksFromHTML encapsulates all HTML-specific extraction logic.
func bookmarksFromHTML(f string) ([]*bookmark.Bookmark, error) {
	file, err := os.Open(f)
	if err != nil {
		log.Printf("Error opening file: %v, %q\n", err, f)
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("Err closing file", "file", f)
		}
	}()

	if err := bookio.IsValidNetscapeFile(file); err != nil {
		return nil, err
	}

	bp := bookio.NewHTMLParser()
	nbs, err := bp.ParseHTML(file)
	if err != nil {
		return nil, err
	}

	bs := make([]*bookmark.Bookmark, 0, len(nbs))
	for i := range nbs {
		bs = append(bs, bookio.FromNetscape(&nbs[i]))
	}

	return bs, nil
}

func bookmarksFromFile(ctx context.Context, f string) ([]*bookmark.Bookmark, error) {
	if err := files.ExistsErr(f); err != nil {
		return nil, err
	}

	repo, err := db.New(f)
	if err != nil {
		return nil, err
	}

	bs, err := repo.All(ctx)
	if err != nil {
		return nil, err
	}
	defer repo.Close()

	return bs, nil
}

func printImportHeader(c *ui.Console, header, fromName, toName string, n int) {
	p := c.Palette()
	title := p.BrightGreen.With(p.Bold).
		Sprint("Import Bookmarks " + header)

	subtitle := p.Dim.With(p.Italic).
		Sprint("merge bookmarks into your collection")

	bs := p.BrightGreen.With(p.Bold).Sprint("found") +
		p.Italic.Sprintf(" %d bookmarks found\n", n)

	value := func(s string) string {
		return p.BrightYellow.With(p.Italic).Sprint(
			files.CollapseHomeDir(s),
		)
	}

	c.Frame().
		Headerln(title).
		Headerln(subtitle).
		Rowln().
		Info(txt.PaddedLine("source:", value(toName+"\n"))).
		Info(txt.PaddedLine("destination:", value(fromName+"\n"))).
		Rowln().
		Info(bs).
		Flush()
}

// promptImportSelection runs the interactive action loop.
func promptImportSelection(c *ui.Console, app *application.App, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	for {
		n := len(bs)
		options := []string{"yes", "no"}
		if n > 1 {
			options = append(options, "select")
		}

		opt, err := c.Choose(
			fmt.Sprintf("import %d bookmarks into %q?", n, files.StripSuffixes(app.DBName)),
			options,
			"y",
		)
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			return nil, sys.ErrActionAborted
		case "s", "select":
			c.ClearLine(1)
			m := menu.New[*bookmark.Bookmark](
				menu.WithOutputColor(app.Flags.Color),
				menu.WithConfig(app.Menu),
				menu.WithArgs("--cycle"),
				menu.WithHeader("select record/s to import"),
				menu.WithInterruptFn(c.Term().InterruptFn),
				menu.WithNth("3.."),
				menu.WithMultiSelection(),
			)
			m.SetFormatter(func(b **bookmark.Bookmark) string { return formatter.OnelineFunc(c, *b) })
			bs, err = m.Select(bs)
			if err != nil {
				return nil, err
			}
		case "y", "yes":
			return bs, nil
		}
	}
}
