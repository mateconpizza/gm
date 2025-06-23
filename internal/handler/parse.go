package handler

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/scraper"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

type bookmarkTemp struct {
	title, desc, tags, favicon string
}

// NewBookmark fetch metadata and parses the new bookmark.
func NewBookmark(
	c *ui.Console,
	r *db.SQLiteRepository,
	b *bookmark.Bookmark,
	title, tags string,
	args []string,
) error {
	newURL, err := newURLFromArgs(c, args)
	if err != nil {
		return err
	}

	newURL = strings.TrimRight(newURL, "/")
	if b, exists := r.Has(newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrDuplicate, b.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(newURL, scraper.WithSpinner("scraping webpage..."))

	// fetch title, description and tags
	fetchTitleAndDesc(c, sc, bTemp)
	tagsFromArgs(c, sc, bTemp)

	b.URL = newURL
	b.Title = bTemp.title
	b.Desc = strings.Join(txt.SplitIntoChunks(bTemp.desc, terminal.MinWidth), "\n")
	b.Tags = bookmark.ParseTags(bTemp.tags)
	b.FaviconURL = bTemp.favicon

	return nil
}

// readURLFromClipboard checks if there a valid URL in the clipboard.
func readURLFromClipboard(c *ui.Console) string {
	cb := sys.ReadClipboard()
	if !validURL(cb) {
		return ""
	}

	c.F.Mid(color.BrightMagenta("URL\t:").String()).
		Textln(" " + color.Gray(cb).String())

	lines := txt.CountLines(c.F.String())
	c.F.Flush()

	if err := c.ConfirmErr("found valid URL in clipboard, use URL?", "y"); err != nil {
		c.T.ClearLine(lines)
		return ""
	}

	c.T.ClearLine(1)

	return cb
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(c *ui.Console, args []string) (string, error) {
	cm := func(s string) string { return color.BrightMagenta(s).String() }
	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight((args)[0], "\n")
		c.F.Header(cm("URL\t:")).
			Text(" " + color.Gray(bURL).String()).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(c)
	if cb != "" {
		return cb, nil
	}

	c.F.Header(cm("URL\t:")).Flush()

	bURL := c.T.Input(" ")
	if bURL == "" {
		return bURL, bookmark.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	cb := func(s string) string { return color.BrightBlue(s).String() }
	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }

	c.F.Header(cb("Tags\t:"))

	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		c.F.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	_ = sc.Start()

	keywords, _ := sc.Keywords()
	if keywords != "" {
		tt := bookmark.ParseTags(keywords)
		b.tags = tt
		c.F.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	if config.App.Flags.Force {
		b.tags = "notag"
		c.F.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	// prompt|take input for tags
	c.F.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()

	mTags, _ := db.TagsCounterFromPath(config.App.DBPath)
	b.tags = bookmark.ParseTags(c.T.ChooseTags(c.F.Border.Mid, mTags))

	c.F.Reset().Mid(cb("Tags\t:")).Textln(" " + cgi(b.tags))

	c.T.ClearLine(txt.CountLines(c.F.String()))
	c.F.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	const indentation int = 10

	width := terminal.MinWidth - len(c.F.Border.Row)

	cc := func(s string) string { return color.BrightCyan(s).String() }
	cg := func(s string) string { return color.BrightGray(s).String() }
	co := func(s string) string { return color.BrightOrange(s).String() }

	if b.title != "" {
		t := cg(txt.SplitAndAlign(b.title, width, indentation))
		c.F.Mid(cc("Title\t: ")).Textln(t).Flush()

		return
	}

	// scrape data
	_ = sc.Start()
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.tags, _ = sc.Keywords()
	b.favicon, _ = sc.Favicon()

	// title
	t := cg(txt.SplitAndAlign(b.title, width, indentation))
	c.F.Mid(cc("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := cg(txt.SplitAndAlign(b.desc, width, indentation))
		c.F.Mid(co("Desc\t: ")).Textln(descColor)
	}

	c.F.Flush()
}
