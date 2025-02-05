package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/browser"
	"github.com/haaag/gm/internal/browser/blink"
	"github.com/haaag/gm/internal/browser/gecko"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
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
//   - ...
func getBrowser(key string) (browser.Browser, bool) {
	for _, pair := range registeredBrowser {
		if pair.key == key {
			return pair.browser, true
		}
	}

	return nil, false
}

// loadBrowser loads the browser paths for the import process.
func loadBrowser(k string) (browser.Browser, error) {
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
	if bs.Len() == 0 {
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

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sc := scraper.New(b.URL, scraper.WithContext(ctx))
			if err := sc.Scrape(); err != nil {
				log.Printf("scraping error: %v", err)
			}

			b.Desc = sc.Desc()

			mu.Lock()
			r.Append(b)
			mu.Unlock()
		}(b)
	})

	wg.Wait()

	*bs = *r

	return nil
}

// importFromBrowser imports bookmarks from a browser.
func importFromBrowser(t *terminal.Term, r *Repo) error {
	name := selectBrowser(t)
	br, err := loadBrowser(name)
	if err != nil {
		return err
	}
	// find bookmarks
	bs, err := br.Import(t)
	if err != nil {
		return fmt.Errorf("browser '%s': %w", br.Name(), err)
	}
	// clean and process found bookmarks
	if err := parseRecordsFound(t, r, bs); err != nil {
		return err
	}
	if bs.Len() == 0 {
		return nil
	}

	return insertRecordsFromSource(t, r, bs)
}

// selectBrowser returns the name of the browser selected by the user.
func selectBrowser(t *terminal.Term) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header("Supported Browsers").Ln().Row().Ln()

	for _, c := range registeredBrowser {
		b := c.browser
		f.Mid(b.Color(b.Short()) + " " + b.Name()).Ln()
	}
	f.Row().Ln().Footer("which browser do you use?").Render()

	name := t.Prompt(" ")
	t.ClearLine(format.CountLines(f.String()))

	return name
}

// parseRecordsFound processes the bookmarks found from the import
// browser process.
func parseRecordsFound(t *terminal.Term, r *Repo, bs *Slice) error {
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
