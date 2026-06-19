package port

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// browsers the list of supported browsers.
func browsers() []browser.Supported {
	return append(gecko.Supported, blink.Supported...)
}

// Browser imports bookmarks from a supported browser.
func Browser(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	br, err := selectBrowser(ctx, app, d.Console())
	if err != nil {
		return err
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}

	// find bookmarks
	bs, err := br.Import(ctx, d.Console(), app.Flags.Yes)
	if err != nil {
		return fmt.Errorf("import from browser %q: %w", strings.ToLower(br.Name()), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(ctx, d, bs)
	if err != nil {
		return err
	}

	return importPipeline(ctx, d, "from browser", br.Name(), bs)
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	c := d.Console()

	r, err := d.Repository()
	if err != nil {
		return nil, err
	}

	bs, err = DeduplicateReport(ctx, c, r, bs)
	if err != nil {
		return nil, err
	}

	if len(bs) == 0 {
		p := c.Palette()
		c.Frame().Error("no new bookmark found, ").
			Textln(p.BrightYellow.Wrap("skipping import", p.Italic)).
			Flush()
		return bs, sys.ErrExitFailure
	}

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	if !app.Flags.Yes &&
		!c.Confirm(ctx, fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs)), "n") {
		return bs, nil
	}

	if err := metadata.ScrapeDescriptions(ctx, bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}

// selectBrowser returns the key of the browser selected by the user.
func selectBrowser(ctx context.Context, app *application.App, c *ui.Console) (browser.Browser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p := c.Palette()
	title := p.BrightGreen.With(p.Bold).
		Sprint("Import Bookmarks from Browser")

	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	subtitle := p.Dim.With(p.Italic).
		Sprint("merge bookmarks into your collection")

	c.Frame().Headerln(title + comment).
		Headerln(subtitle)

	m := picker.New[browser.Supported](app)
	browsers, err := m.Select(browsers())
	if err != nil {
		return nil, err
	}

	selected := browsers[0]

	return selected.Browser, nil
}
