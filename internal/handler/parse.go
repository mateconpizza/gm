package handler

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
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
func NewBookmark(d *deps.Deps, b *bookmark.Bookmark, args []string) error {
	app, err := d.Application()
	if err != nil {
		return err
	}
	title := app.Flags.Title
	tags := app.Flags.TagsStr
	c := d.Console()
	newURL, err := newURLFromArgs(c, args)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	if b, exists := r.Has(d.Context(), newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrBookmarkDuplicate, b.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(newURL, scraper.WithContext(d.Context()), scraper.WithSpinner("scraping webpage..."))

	// fetch title, description and tags
	fetchTitleAndDesc(c, sc, bTemp)
	if err := tagsFromArgs(d, sc, bTemp); err != nil {
		return err
	}

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
	if !ValidURL(cb) {
		return ""
	}

	f, p := c.Frame(), c.Palette()
	dot := func() string { return p.BrightMagenta.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }
	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
		Textln(" " + p.Gray.Sprint(cb))

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
	dot := func() string { return p.BrightMagenta.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }

	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight(args[0], "\n")
		f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
			Text(" " + p.Gray.Sprint(bURL)).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(c)
	if cb != "" {
		return cb, nil
	}

	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).Flush()

	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, metadata.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(d *deps.Deps, sc *scraper.Scraper, b *bookmarkTemp) error {
	c := d.Console()
	f, p := c.Frame(), c.Palette()

	dot := func() string { return p.BrightBlue.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }
	f.CustomFunc(dot, p.BrightBlue.Sprint("Tags\t:"))

	// Use existing tags if provided
	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()
		return nil
	}

	// Try to get keywords from scraper
	_ = sc.Start()
	if keywords, _ := sc.Keywords(); keywords != "" {
		b.tags = bookmark.ParseTags(keywords)
		f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()
		return nil
	}

	// Use default if force flag is set
	app, err := d.Application()
	if err != nil {
		return err
	}
	if app.Flags.Force {
		b.tags = bookmark.DefaultTag
		f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()
		return nil
	}

	// Display prompt for tag input format
	f.Text(p.Gray.Sprint(" (spaces|comma separated)\n")).Flush()

	// Get existing tags from database with their usage counts
	r, err := d.Repository()
	if err != nil {
		return err
	}
	mTags, _ := r.TagsCounter(d.Context())

	// Let user select tags and parse them into proper format
	tags := c.Term().ChooseTags(txt.GlyphTriangleRight.Prefix(" "), mTags)
	b.tags = bookmark.ParseTags(tags)

	// Clear and display the selected tags
	f.Reset().
		CustomFunc(dot, p.BrightBlue.Sprint("Tags\t:")).
		Textln(" " + p.Gray.Wrap(b.tags, p.Italic))

	// Clear previous input lines from terminal
	c.ClearLine(txt.CountLines(f.String()))
	f.Flush()
	return nil
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	f, p := c.Frame(), c.Palette()
	const indentation int = 10

	borders := f.Borders()
	width := terminal.MinWidth - len(borders.Row)
	dot := func() string { return p.BrightCyan.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }

	if b.title != "" {
		t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
		f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t).Flush()

		return
	}

	// scrape data
	_ = sc.Start()
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.tags, _ = sc.Keywords()
	b.favicon, _ = sc.Favicon()

	if b.tags == "" {
		tags, _ := sc.TagsRepo()
		b.tags = strings.Join(tags, ",")
	}

	// title
	t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
	f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := p.Gray.Sprint(txt.SplitAndAlign(b.desc, width, indentation))
		dot := func() string { return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }
		f.CustomFunc(dot, p.BrightYellow.Sprint("Desc\t: ")).Textln(descColor)
	}

	f.Flush()
}
