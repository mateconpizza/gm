package handler

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

type bookmarkTemp struct {
	title, desc, tags, favicon string
}

// NewBookmark fetch metadata and parses the new bookmark.
func NewBookmark(a *app.Context, b *bookmark.Bookmark, args []string) error {
	title := a.Cfg.Flags.Title
	tags := a.Cfg.Flags.TagsStr
	newURL, err := newURLFromArgs(a.Console, args)
	if err != nil {
		return err
	}

	newURL = strings.TrimRight(newURL, "/")
	if b, exists := a.DB.Has(a.Ctx, newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrBookmarkDuplicate, b.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(newURL, scraper.WithContext(a.Ctx), scraper.WithSpinner("scraping webpage..."))

	// fetch title, description and tags
	fetchTitleAndDesc(a.Console, sc, bTemp)
	tagsFromArgs(a, sc, bTemp)

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

	c.Frame.Mid(color.BrightMagenta("URL\t:").String()).
		Textln(" " + color.Gray(cb).String())

	lines := txt.CountLines(c.Frame.String())
	c.Frame.Flush()

	if err := c.ConfirmErr("found valid URL in clipboard, use URL?", "y"); err != nil {
		c.Term.ClearLine(lines)
		return ""
	}

	c.Term.ClearLine(1)

	return cb
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(c *ui.Console, args []string) (string, error) {
	cm := func(s string) string { return color.BrightMagenta(s).String() }
	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight((args)[0], "\n")
		c.Frame.Header(cm("URL\t:")).
			Text(" " + color.Gray(bURL).String()).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(c)
	if cb != "" {
		return cb, nil
	}

	c.Frame.Header(cm("URL\t:")).Flush()

	bURL := c.Term.Input(" ")
	if bURL == "" {
		return bURL, metadata.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(a *app.Context, sc *scraper.Scraper, b *bookmarkTemp) {
	cb := func(s string) string { return color.BrightBlue(s).String() }
	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }

	a.Console.Frame.Header(cb("Tags\t:"))

	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		a.Console.Frame.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	_ = sc.Start()

	keywords, _ := sc.Keywords()
	if keywords != "" {
		tt := bookmark.ParseTags(keywords)
		b.tags = tt
		a.Console.Frame.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	if a.Cfg.Flags.Force {
		b.tags = "notag"
		a.Console.Frame.Textln(" " + cgi(b.tags)).Flush()

		return
	}

	// prompt|take input for tags
	a.Console.Frame.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()

	mTags, _ := dbtask.TagsCounterFromPath(a.Ctx, a.Cfg.DBPath)
	b.tags = bookmark.ParseTags(a.Console.Term.ChooseTags(a.Console.Frame.Border.Mid, mTags))

	a.Console.Frame.Reset().Mid(cb("Tags\t:")).Textln(" " + cgi(b.tags))

	a.Console.ClearLine(txt.CountLines(a.Console.Frame.String()))
	a.Console.Frame.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	const indentation int = 10

	width := terminal.MinWidth - len(c.Frame.Border.Row)

	cc := func(s string) string { return color.BrightCyan(s).String() }
	cg := func(s string) string { return color.BrightGray(s).String() }
	co := func(s string) string { return color.BrightOrange(s).String() }

	if b.title != "" {
		t := cg(txt.SplitAndAlign(b.title, width, indentation))
		c.Frame.Mid(cc("Title\t: ")).Textln(t).Flush()

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
	c.Frame.Mid(cc("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := cg(txt.SplitAndAlign(b.desc, width, indentation))
		c.Frame.Mid(co("Desc\t: ")).Textln(descColor)
	}

	c.Frame.Flush()
}
