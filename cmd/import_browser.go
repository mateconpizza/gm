package cmd

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/browser"
	"github.com/haaag/gm/internal/browser/blink"
	"github.com/haaag/gm/internal/browser/gecko"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

// supportedBrowser defines a supported browser.
type supportedBrowser struct {
	key     string
	browser browser.Browser
}

// registeredBrowser the list of supported browsers.
var registeredBrowser = []supportedBrowser{
	{"f", gecko.New("Firefox", color.BrightOrange)},
	{"z", gecko.New("Zen", color.BrightBlack)},
	{"w", gecko.New("Waterfox", color.BrightBlue)},
	{"c", blink.New("Chromium", color.BrightBlue)},
	{"g", blink.New("Google Chrome", color.BrightYellow)},
	{"b", blink.New("Brave", color.BrightOrange)},
	{"v", blink.New("Vivaldi", color.BrightRed)},
	{"e", blink.New("Edge", color.BrightCyan)},
}

// getBrowser returns a browser by its short key.
//
// key: the first letter of the browser name.
//   - Firefox -> f
//   - Waterfox -> w
//   - Chromium -> c
func getBrowser(key string) (browser.Browser, bool) {
	for _, pair := range registeredBrowser {
		if pair.key == key {
			return pair.browser, true
		}
	}

	return nil, false
}

// importLoadBrowser loads the browser paths for the import process.
func importLoadBrowser(k string) (browser.Browser, error) {
	b, ok := getBrowser(k)
	if !ok {
		return nil, fmt.Errorf("%w: '%s'", browser.ErrBrowserUnsupported, k)
	}
	if err := b.LoadPaths(); err != nil {
		return nil, fmt.Errorf("error loading browser paths: %w", err)
	}

	return b, nil
}

// scrapeMissingData scrapes missing data from bookmarks found from the import
// process.
func scrapeMissingData(bs *Slice) error {
	n := bs.Len()
	if n == 0 {
		return nil
	}

	msg := color.BrightGreen("scraping missing data...").Italic().String()
	sp := spinner.New(spinner.WithColor(color.Gray), spinner.WithMesg(msg))
	sp.Start()
	defer sp.Stop()

	var (
		r  = slice.New[Bookmark]()
		mu sync.Mutex
		wg sync.WaitGroup
	)

	bs.ForEach(func(b Bookmark) {
		wg.Add(1)
		go func(b Bookmark) {
			defer wg.Done()

			sc := scraper.New(b.URL)
			err := sc.Scrape()
			if err != nil {
				log.Printf("scraping error: %v", err)
			}

			b.Desc = sc.Desc()

			mu.Lock()
			r.Append(&b)
			mu.Unlock()
		}(b)
	})

	wg.Wait()

	*bs = *r

	return nil
}

// importFromBrowser imports bookmarks from a browser.
func importFromBrowser(t *terminal.Term, r *repo.SQLiteRepository) error {
	var b browser.Browser
	var err error
	name := importSelectBrowser(t)
	if b, err = importLoadBrowser(name); err != nil {
		return err
	}

	// find bookmarks
	bs, err := b.Import(t)
	if err != nil {
		return fmt.Errorf("browser '%s': %w", b.Name(), err)
	}
	// clean and process found bookmarks
	if err := importParseRecordsFound(t, r, bs); err != nil {
		return err
	}

	return importInsert(r, bs)
}

// importSelectBrowser returns the name of the browser selected by the user.
func importSelectBrowser(t *terminal.Term) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header("Supported Browsers").Ln().Row().Ln()

	for _, c := range registeredBrowser {
		b := c.browser
		f.Mid(b.Color(b.Short()) + " " + b.Name()).Ln()
	}
	f.Row().Ln().Footer("which browser do you use?").Render()

	l := format.CountLines(f.String())
	name := t.Prompt(" ")
	t.ClearLine(l)

	return name
}

// importParseRecordsFound processes the bookmarks found from the import
// browser process.
func importParseRecordsFound(t *terminal.Term, r *repo.SQLiteRepository, bs *Slice) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if err := cleanDuplicateRecords(r, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Row().Ln().Mid("no new bookmark found, skipping import").Ln().Render()
			return nil
		}
	}

	countStr := color.BrightBlue(bs.Len())
	msg := fmt.Sprintf("scrape missing data from %s bookmarks found?", countStr)
	f.Row().Ln().Render().Clean()
	if t.Confirm(f.Mid(msg).String(), "n") {
		if err := scrapeMissingData(bs); err != nil {
			return err
		}
	}

	return nil
}

// importBrowserCmd imports bookmarks from a browser.
var importBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "import bookmarks from browser",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		return importFromBrowser(t, r)
	},
}

func init() {
	// add cmd as a `import` subcommand
	importCmd.AddCommand(importBrowserCmd)
}
