package port

import (
	"context"
	"fmt"

	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// supportedBrowser represents a supported browser.
type supportedBrowser struct {
	key     string
	browser browser.Browser
}

// registeredBrowser the list of supported browsers.
var registeredBrowser = []supportedBrowser{
	{"f", gecko.New("Firefox", ansi.BrightYellow.With(ansi.Bold))},
	{"z", gecko.New("Zen", ansi.Red.With(ansi.Bold))},
	{"w", gecko.New("Waterfox", ansi.BrightBlue.With(ansi.Bold))},
	{"c", blink.New("Chromium", ansi.BrightBlue.With(ansi.Bold))},
	{"g", blink.New("Google Chrome", ansi.BrightYellow.With(ansi.Bold))},
	{"b", blink.New("Brave", ansi.Magenta.With(ansi.Bold))},
	{"v", blink.New("Vivaldi", ansi.BrightRed.With(ansi.Bold))},
	{"e", blink.New("Edge", ansi.BrightCyan.With(ansi.Bold))},
}

// Browser imports bookmarks from a supported browser.
func Browser(ctx context.Context, d *deps.Deps) error {
	br, ok := getBrowser(selectBrowser(d.Console()))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}

	// find bookmarks
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	bs, err := br.Import(d.Console(), app.Flags.Yes)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(ctx, d, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
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

	if !app.Flags.Yes && !c.Confirm(fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs)), "y") {
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

	for _, pair := range registeredBrowser {
		if pair.key == key {
			return pair.browser, true
		}
	}

	return nil, false
}

// selectBrowser returns the key of the browser selected by the user.
func selectBrowser(c *ui.Console) string {
	f, p := c.Frame(), c.Palette()
	title := p.BrightGreen.With(p.Bold).
		Sprint("Import Bookmarks from Browser")

	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	subtitle := p.Dim.With(p.Italic).
		Sprint("merge bookmarks into your collection")

	f.Headerln(title + comment).
		Headerln(subtitle).
		Rowln().Flush().
		Midln("Supported Browsers").
		Rowln()

	for _, browser := range registeredBrowser {
		b := browser.browser
		f.Midln(
			b.Color(fmt.Sprintf("[%s]", b.Short())) +
				" " +
				b.Name(),
		)
	}

	defer c.ClearLine(txt.CountLines(f.String()) + 1)
	f.Rowln().Flush()

	return c.Prompt("Select browser: ")
}
