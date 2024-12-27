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
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrBrowserUnsupported = errors.New("browser is unsupported")

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

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import bookmarks from browser",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return verifyDatabase(Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		f.Header("Supported Browsers").Ln().Row().Ln()

		for _, c := range registeredBrowser {
			b := c.browser
			f.Mid(b.Color(b.Short()) + " " + b.Name()).Ln()
		}
		f.Row().Ln().Footer("which browser do you use?").Render()

		l := format.CountLines(f.String())
		name := terminal.Prompt(" ")
		terminal.ClearLine(l)

		b, ok := getBrowser(name)
		if !ok {
			return fmt.Errorf("%w: '%s'", ErrBrowserUnsupported, name)
		}

		if err := b.LoadPaths(); err != nil {
			return fmt.Errorf("error loading browser paths: %w", err)
		}

		bs, err := b.Import()
		if err != nil {
			return fmt.Errorf("importing from '%s': %w", b.Name(), err)
		}

		ogLen := bs.Len()
		bs.Filter(func(b Bookmark) bool {
			return !r.HasRecord(r.Cfg.TableMain, "url", b.URL)
		})

		f.Clean()
		if ogLen != bs.Len() {
			skip := color.BrightYellow("skipping")
			s := fmt.Sprintf("%s %d duplicate bookmarks found", skip, ogLen-bs.Len())
			f.Row().Ln().Mid(s).Ln()
		}

		if bs.Len() == 0 {
			f.Row().Ln().Mid("no new bookmark found, skipping import").Ln().Render()
			return nil
		}

		countStr := color.BrightBlue(bs.Len())
		msg := fmt.Sprintf("scrape missing data from %s bookmarks found?", countStr)
		f.Row().Ln().Mid(msg).Render()
		if terminal.Confirm("", "n") {
			if err := scrapeMissingData(bs); err != nil {
				return err
			}
		}

		bs.ForEach(func(b Bookmark) {
			_, _ = r.Insert(r.Cfg.TableMain, &b)
		})

		success := color.BrightGreen("Successfully").Italic().Bold()
		f.Clean().Row().Ln()
		f.Footer(fmt.Sprintf("%s %d bookmarks imported.\n", success, bs.Len())).Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
