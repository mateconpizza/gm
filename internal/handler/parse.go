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
	c := a.Console()
	newURL, err := newURLFromArgs(c, args)
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
	fetchTitleAndDesc(c, sc, bTemp)
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

	f := c.Frame()
	f.Mid(c.Palette().BrightMagenta("URL\t:")).Textln(" " + c.Palette().Gray(cb))

	lines := txt.CountLines(f.String())
	f.Flush()

	t := c.Term()
	if err := c.ConfirmErr("found valid URL in clipboard, use URL?", "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return cb
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(c *ui.Console, args []string) (string, error) {
	f, t, p := c.Frame(), c.Term(), c.Palette()

	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight((args)[0], "\n")
		f.Header(p.BrightMagenta("URL\t:")).
			Text(" " + p.Gray(bURL)).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(c)
	if cb != "" {
		return cb, nil
	}

	f.Header(p.BrightMagenta("URL\t:")).Flush()

	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, metadata.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(a *app.Context, sc *scraper.Scraper, b *bookmarkTemp) {
	c := a.Console()
	f, p := c.Frame(), c.Palette()

	f.Header(p.BrightBlue("Tags\t:"))

	// Use existing tags if provided
	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Textln(" " + p.BrightGrayItalic(b.tags)).Flush()
		return
	}

	// Try to get keywords from scraper
	_ = sc.Start()
	if keywords, _ := sc.Keywords(); keywords != "" {
		b.tags = bookmark.ParseTags(keywords)
		f.Textln(" " + p.BrightGrayItalic(b.tags)).Flush()
		return
	}

	// Use default if force flag is set
	if a.Cfg.Flags.Force {
		b.tags = "notag"
		f.Textln(" " + p.BrightGrayItalic(b.tags)).Flush()
		return
	}

	// Prompt user for tags
	f.Text(p.Gray(" (spaces|comma separated)")).Ln().Flush()
	mTags, _ := dbtask.TagsCounterFromPath(a.Ctx, a.Cfg.DBPath)
	b.tags = bookmark.ParseTags(c.Term().ChooseTags(f.Border.Mid, mTags))
	f.Reset().Mid(p.BrightBlue("Tags\t:")).Textln(" " + p.BrightGrayItalic(b.tags))
	c.ClearLine(txt.CountLines(f.String()))
	f.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	f, p := c.Frame(), c.Palette()
	const indentation int = 10

	width := terminal.MinWidth - len(f.Border.Row)

	if b.title != "" {
		t := p.BrightGray(txt.SplitAndAlign(b.title, width, indentation))
		f.Mid(p.BrightCyan("Title\t: ")).Textln(t).Flush()

		return
	}

	// scrape data
	_ = sc.Start()
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.tags, _ = sc.Keywords()
	b.favicon, _ = sc.Favicon()

	// title
	t := p.BrightGray(txt.SplitAndAlign(b.title, width, indentation))
	f.Mid(p.BrightCyan("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := p.BrightGray(txt.SplitAndAlign(b.desc, width, indentation))
		f.Mid(p.BrightOrange("Desc\t: ")).Textln(descColor)
	}

	f.Flush()
}
