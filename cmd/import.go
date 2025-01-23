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
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	importBrowserFlag bool

	// importFromDBFlag is used to import bookmarks from another database.
	importFromDBFlag bool
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

// importSelectionMenu returns the name of the browser selected by the user.
func importSelectionMenu(t *terminal.Term) string {
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

// importRemoveDuplicates removes duplicate bookmarks from the import process.
func importRemoveDuplicates(r *repo.SQLiteRepository, bs *Slice) error {
	ogLen := bs.Len()
	bs.Filter(func(b Bookmark) bool {
		return !r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	})

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if ogLen != bs.Len() {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, ogLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Render()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// importProcessFound processes the bookmarks found from the import process.
func importProcessFound(t *terminal.Term, r *repo.SQLiteRepository, bs *Slice) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if err := importRemoveDuplicates(r, bs); err != nil {
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

// importInsert inserts the bookmarks found from the import process.
func importInsert(r *repo.SQLiteRepository, bs *Slice) error {
	if bs.Empty() {
		return nil
	}

	insert := func(b Bookmark) error {
		err := r.Insert(&b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	}

	if err := bs.ForEachErr(insert); err != nil {
		return fmt.Errorf("inserting found bookmark: %w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	success := color.BrightGreen("Successfully").Italic().Bold()
	f.Row().Ln()
	f.Success(fmt.Sprintf("%s %d bookmarks imported.\n", success, bs.Len())).Render()

	return nil
}

// importFromBrowser imports bookmarks from a browser.
func importFromBrowser(t *terminal.Term, r *repo.SQLiteRepository) error {
	var b browser.Browser
	var err error
	name := importSelectionMenu(t)
	if b, err = importLoadBrowser(name); err != nil {
		return err
	}

	// find bookmarks
	bs, err := b.Import(t)
	if err != nil {
		return fmt.Errorf("importing from '%s': %w", b.Name(), err)
	}
	// clean and process found bookmarks
	if err := importProcessFound(t, r, bs); err != nil {
		return err
	}

	return importInsert(r, bs)
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import bookmarks from browser",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return handler.ValidateDB(cmd, Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		flags := map[bool]func(t *terminal.Term, r *repo.SQLiteRepository) error{
			importBrowserFlag: importFromBrowser,
		}
		if handler, ok := flags[true]; ok {
			return handler(t, r)
		}

		return nil
	},
}

func init() {
	importCmd.Flags().BoolVar(&importBrowserFlag, "browser", true, "import from browser")
	importCmd.Flags().BoolVarP(&importFromDBFlag, "from", "F", false, "import from database")
	rootCmd.AddCommand(importCmd)
}
