package port

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// browsers the list of supported browsers.
func browsers() []browser.Supported {
	r := make([]browser.Supported, 0, len(gecko.Supported)+len(blink.Supported))

	r = append(r, gecko.Supported...)
	r = append(r, blink.Supported...)

	return r
}

// Browser imports bookmarks from a supported browser.
func Browser(ctx context.Context, d *deps.Deps) error {
	selected, err := selectBrowser(ctx, d.Console())
	if err != nil {
		return err
	}

	br, ok := getBrowser(selected)
	if !ok {
		return fmt.Errorf("%w: %q", browser.ErrBrowserUnsupported, selected)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}

	// find bookmarks
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	bs, err := br.Import(ctx, d.Console(), app.Flags.Yes)
	if err != nil {
		return fmt.Errorf("import from browser %q: %w", strings.ToLower(br.Name()), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(ctx, d, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return sys.ErrExitFailure
	}

	return IntoRepo(ctx, d, bs)
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	c, f := d.Console(), d.Console().Frame()
	r, err := d.Repository()
	if err != nil {
		return nil, err
	}
	bs, err = DeduplicateReport(ctx, c, r, bs)
	if err != nil {
		return nil, err
	}

	if len(bs) == 0 {
		f.Midln("no new bookmark found, skipping import").Flush()
		return bs, nil
	}

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	if !app.Flags.Yes &&
		!c.Confirm(ctx, fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs)), "y") {
		return bs, nil
	}

	if err := metadata.ScrapeDescriptions(ctx, bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}

// getBrowser returns a browser by its short key.
//
// key: the first letter of the browser name.
//   - Firefox -> f
//   - Waterfox -> w
//   - Chromium -> c
//   - ...
func getBrowser(key string) (browser.Browser, bool) {
	if key == "" {
		return nil, false
	}

	for _, b := range browsers() {
		if b.Browser.Short() == key {
			return b.Browser, true
		}
	}

	return nil, false
}

// selectBrowser returns the key of the browser selected by the user.
func selectBrowser(ctx context.Context, c *ui.Console) (string, error) {
	p := c.Palette()
	title := p.BrightGreen.With(p.Bold).
		Sprint("Import Bookmarks from Browser")

	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	subtitle := p.Dim.With(p.Italic).
		Sprint("merge bookmarks into your collection")

	f := c.Frame()
	f.Headerln(title + comment).
		Headerln(subtitle).
		Rowln().
		Midln("Supported Browsers").
		Rowln()

	for _, browser := range browsers() {
		b := browser.Browser
		key := b.Color(fmt.Sprintf("[%s] ", b.Short()))
		f.Midln(key + b.Name())
	}

	f.Rowln().Flush()
	selected, err := c.Prompt(ctx, "Select browser: ")
	if err != nil {
		return "", err
	}

	return selected, nil
}
