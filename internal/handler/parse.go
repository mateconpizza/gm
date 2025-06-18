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
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

type bookmarkTemp struct {
	title, desc, tags string
}

// NewBookmark fetch metadata and parses the new bookmark.
func NewBookmark(
	f *frame.Frame,
	t *terminal.Term,
	r *db.SQLiteRepository,
	b *bookmark.Bookmark,
	title, tags string,
	args []string,
) error {
	newURL, err := newURLFromArgs(t, f, args)
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

	sc := scraper.New(newURL, scraper.WithSpinner())

	// fetch title, description and tags
	fetchTitleAndDesc(f.Reset(), sc, bTemp)
	tagsFromArgs(t, f.Reset(), sc, bTemp)

	b.URL = newURL
	b.Title = bTemp.title
	b.Desc = strings.Join(txt.SplitIntoChunks(bTemp.desc, terminal.MinWidth), "\n")
	b.Tags = bookmark.ParseTags(bTemp.tags)

	return nil
}

// readURLFromClipboard checks if there a valid URL in the clipboard.
func readURLFromClipboard(t *terminal.Term, f *frame.Frame) string {
	c := sys.ReadClipboard()
	if !validURL(c) {
		return ""
	}

	f.Mid(color.BrightMagenta("URL\t:").String()).
		Textln(" " + color.Gray(c).String())

	lines := txt.CountLines(f.String())
	f.Flush()

	if err := t.ConfirmErr(f.Reset().Question("found valid URL in clipboard, use URL?").String(), "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return c
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(t *terminal.Term, f *frame.Frame, args []string) (string, error) {
	cm := func(s string) string { return color.BrightMagenta(s).String() }
	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight((args)[0], "\n")
		f.Header(cm("URL\t:")).
			Text(" " + color.Gray(bURL).String()).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	c := readURLFromClipboard(t, f.Reset())
	if c != "" {
		return c, nil
	}

	f.Reset().Header(cm("URL\t:")).Flush()
	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, bookmark.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(t *terminal.Term, f *frame.Frame, sc *scraper.Scraper, b *bookmarkTemp) {
	cb := func(s string) string { return color.BrightBlue(s).String() }
	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }

	f.Header(cb("Tags\t:"))
	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Textln(" " + cgi(b.tags)).Flush()
		return
	}

	_ = sc.Start()
	keywords, _ := sc.Keywords()
	if keywords != "" {
		tt := bookmark.ParseTags(keywords)
		b.tags = tt
		f.Textln(" " + cgi(b.tags)).Flush()
		return
	}

	if config.App.Force {
		b.tags = "notag"
		f.Textln(" " + cgi(b.tags)).Flush()
		return
	}

	// prompt|take input for tags
	f.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()
	mTags, _ := db.TagsCounterFromPath(config.App.DBPath)
	b.tags = bookmark.ParseTags(t.ChooseTags(f.Border.Mid, mTags))

	f.Reset().Mid(cb("Tags\t:")).Textln(" " + cgi(b.tags))

	t.ClearLine(txt.CountLines(f.String()))
	f.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(f *frame.Frame, sc *scraper.Scraper, b *bookmarkTemp) {
	const indentation int = 10
	width := terminal.MinWidth - len(f.Border.Row)

	cc := func(s string) string { return color.BrightCyan(s).String() }
	cg := func(s string) string { return color.BrightGray(s).String() }
	co := func(s string) string { return color.BrightOrange(s).String() }

	if b.title != "" {
		t := cg(txt.SplitAndAlign(b.title, width, indentation))
		f.Mid(cc("Title\t: ")).Textln(t).Flush()
		return
	}

	// scrape data
	_ = sc.Start()
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.tags, _ = sc.Keywords()

	// title
	t := cg(txt.SplitAndAlign(b.title, width, indentation))
	f.Mid(cc("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := cg(txt.SplitAndAlign(b.desc, width, indentation))
		f.Mid(co("Desc\t: ")).Textln(descColor)
	}

	f.Flush()
}
