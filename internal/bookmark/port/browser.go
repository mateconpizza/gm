package port

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// ImportFromBrowser imports bookmarks from a supported browser.
func ImportFromBrowser(args []string) func(ctx context.Context, d *deps.Deps) error {
	return func(ctx context.Context, d *deps.Deps) error {
		app, err := d.Application(ctx)
		if err != nil {
			return err
		}

		opLog := app.Logger().With("op", "import-browser")
		log := opLog.With("cmp", "port", "fn", "import-browser")

		b := newBrowser(app.Flags.Path, app.Flags.BrowserType, log)

		br := b
		if br == nil {
			if br, err = chooseBrowser(ctx, app, args, log); err != nil {
				return err
			}
		}

		if err := br.LoadPaths(log); err != nil {
			return fmt.Errorf("load browser paths: %w", err)
		}

		printImportBanner(d, app)

		bs, err := readBrowserBookmarks(ctx, app, opLog, d.Console(), br)
		if err != nil {
			return err
		}

		bs, err = parseFoundInBrowser(ctx, d, bs)
		if err != nil {
			return fmt.Errorf("process imported bookmarks: %w", err)
		}

		log.Debug("browser bookmarks processed", "browser", br.Name(), "count", len(bs))

		return importPipeline(ctx, d, "browser", br.Name(), bs)
	}
}

func chooseBrowser(ctx context.Context, app *application.App, args []string, log *slog.Logger) (browser.Browser, error) {
	sel, err := selectBrowser(ctx, app, args, SupportedBrowsers())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Debug("browser selection canceled")
		}
		return nil, err
	}

	return sel.Browser, nil
}

func readBrowserBookmarks(ctx context.Context, app *application.App, opLog *slog.Logger, c *ui.Console, b browser.Browser) ([]*bookmark.Bookmark, error) {
	m := setupMenu(app, opLog.With("stage", "bookmark-selection"))

	bs, err := b.Import(ctx, opLog.With("stage", "read-browser-bookmarks"), m, c)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, app.Abort()
		}
		return nil, fmt.Errorf("import from browser %q: %w", strings.ToLower(b.Name()), err)
	}

	return bs, nil
}

func printImportBanner(d *deps.Deps, app *application.App) {
	p := d.Console().Palette()
	d.Console().NewBannerBuilder().
		WithTitle("Import Bookmarks from Browser").
		WithTitleGlyph(app.Glyphs().Add).
		WithTitleColor(p.BrightGreen.With(p.Bold).Sprint).
		WithSubtitle("merge bookmarks into your collection").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln().
		Flush()
}

func SupportedBrowsers() []browser.Supported {
	return append(gecko.Supported, blink.Supported...)
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

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	if len(bs) == 0 {
		c.Frame().Reset().Error("no bookmark found\n").Flush()
		return bs, app.Failure()
	}

	p := c.Palette()
	q := fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs))
	if !c.Confirm(ctx, q, "n") {
		return bs, nil
	}

	sp := rotato.New(
		rotato.WithSpinnerColor(p.Gray.Format),
		rotato.WithMessage("scraping missing data..."),
		rotato.WithPrefixColor(p.Gray.Format),
		rotato.WithMessageColor(p.BrightGreen.With(p.Italic).Format),
		rotato.WithDoneMessageColor(p.BrightGreen.With(p.Italic).Format),
		rotato.WithDoneSymbolColor(p.BrightGreen.With(p.Bold).Format),
	)

	if err := metadata.ScrapeDescriptions(ctx, sp, bs, app.Logger()); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}

// selectBrowser returns the key of the browser selected by the user.
func selectBrowser(ctx context.Context, app *application.App, args []string, browsers []browser.Supported) (browser.Supported, error) {
	if err := ctx.Err(); err != nil {
		return browser.Supported{}, err
	}

	m := picker.New[browser.Supported](
		app,
		menu.WithHeader("select a browser to import from"),
		menu.WithLogger(app.Logger().With("op", "select-browser")),
		menu.WithArgs(func(b *menu.ArgsBuilder) *menu.ArgsBuilder {
			if len(args) > 0 {
				return b.Custom("--query=" + args[0])
			}
			return b
		}),
	)

	browsers, err := m.Select(ctx, browsers)
	if err != nil {
		return browser.Supported{}, err
	}

	selected := browsers[0]

	return selected, nil
}

func setupMenu(app *application.App, l *slog.Logger) *menu.Menu[string] {
	h := "select profile/s to import from " +
		app.Glyphs().Sep +
		" use <TAB> for multi-selection"

	return picker.New[string](
		app,
		menu.WithLogger(l),
		menu.WithHeader(h),
		menu.WithMultiSelection(),
		menu.WithHeaderKeymaps(),
		menu.WithKeybinds(menu.KeymapToggleAll()),
	)
}

func newBrowser(p, t string, l *slog.Logger) browser.Browser {
	l.Debug("user-browser", "path", p, "type", t)
	switch t {
	case gecko.Gecko.String():
		return gecko.Register(filepath.Base(p), p)
	case blink.Blink.String():
		return blink.Register(filepath.Base(p), p)
	default:
		return nil
	}
}
