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
	title := d.Cfg.Flags.Title
	tags := d.Cfg.Flags.TagsStr
	c := d.Console()
	newURL, err := newURLFromArgs(c, args)
	if err != nil {
		return err
	}

	newURL = strings.TrimRight(newURL, "/")
	if b, exists := d.DB.Has(d.Context(), newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrBookmarkDuplicate, b.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(newURL, scraper.WithContext(d.Context()), scraper.WithSpinner("scraping webpage..."))

	// fetch title, description and tags
	fetchTitleAndDesc(c, sc, bTemp)
	tagsFromArgs(d, sc, bTemp)

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

	f, p := c.Frame(), c.Palette()
	f.Mid(p.BrightMagenta.Sprint("URL\t:")).Textln(" " + p.Dim.Sprint(cb))

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
		f.Header(p.BrightMagenta.Sprint("URL\t:")).
			Text(" " + p.Dim.Sprint(bURL)).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(c)
	if cb != "" {
		return cb, nil
	}

	f.Header(p.BrightMagenta.Sprint("URL\t:")).Flush()

	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, metadata.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(d *deps.Deps, sc *scraper.Scraper, b *bookmarkTemp) {
	c := d.Console()
	f, p := c.Frame(), c.Palette()

	f.Header(p.BrightBlue.Sprint("Tags\t:"))

	// Use existing tags if provided
	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Textln(" " + p.Dim.Wrap(b.tags, p.Italic)).Flush()
		return
	}

	// Try to get keywords from scraper
	_ = sc.Start()
	if keywords, _ := sc.Keywords(); keywords != "" {
		b.tags = bookmark.ParseTags(keywords)
		f.Textln(" " + p.Dim.Wrap(b.tags, p.Italic)).Flush()
		return
	}

	// Use default if force flag is set
	if d.Cfg.Flags.Force {
		b.tags = bookmark.DefaultTag
		f.Textln(" " + p.Dim.Wrap(b.tags, p.Italic)).Flush()
		return
	}

	// Display prompt for tag input format
	f.Text(p.Dim.Sprint(" (spaces|comma separated)\n")).Flush()

	// Get existing tags from database with their usage counts
	mTags, _ := d.DB.TagsCounter(d.Context())

	// Let user select tags and parse them into proper format
	b.tags = bookmark.ParseTags(c.Term().ChooseTags(f.Border.Mid, mTags))

	// Clear and display the selected tags
	f.Reset().Mid(p.BrightBlue.Sprint("Tags\t:")).Textln(" " + p.Dim.Wrap(b.tags, p.Italic))

	// Clear previous input lines from terminal
	c.ClearLine(txt.CountLines(f.String()))
	f.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(c *ui.Console, sc *scraper.Scraper, b *bookmarkTemp) {
	f, p := c.Frame(), c.Palette()
	const indentation int = 10

	width := terminal.MinWidth - len(f.Border.Row)

	if b.title != "" {
		t := p.Dim.Sprint(txt.SplitAndAlign(b.title, width, indentation))
		f.Mid(p.BrightCyan.Sprint("Title\t: ")).Textln(t).Flush()

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
	t := p.Dim.Sprint(txt.SplitAndAlign(b.title, width, indentation))
	f.Mid(p.BrightCyan.Sprint("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := p.Dim.Sprint(txt.SplitAndAlign(b.desc, width, indentation))
		f.Mid(p.BrightYellow.Sprint("Desc\t: ")).Textln(descColor)
	}

	f.Flush()
}
