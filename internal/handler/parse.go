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
	tagsFromArgs(t, f.Clear(), sc, bTemp)
	fetchTitleAndDesc(f, sc, bTemp)

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

	if err := t.ConfirmErr(f.Clear().Question("found valid URL in clipboard, use URL?").String(), "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return c
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(t *terminal.Term, f *frame.Frame, args []string) (string, error) {
	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight((args)[0], "\n")
		f.Header(color.BrightMagenta("URL\t:").String()).
			Text(" " + color.Gray(bURL).String()).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	c := readURLFromClipboard(t, f.Clear())
	if c != "" {
		return c, nil
	}

	f.Clear().Header(color.BrightMagenta("URL\t:").String()).Flush()
	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, bookmark.ErrURLEmpty
	}

	return bURL, nil
}

// tagsFromArgs retrieves the Tags from args or prompts the user for input.
func tagsFromArgs(t *terminal.Term, f *frame.Frame, sc *scraper.Scraper, b *bookmarkTemp) {
	f.Header(color.BrightBlue("Tags\t:").String())
	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Text(" " + color.Gray(b.tags).String()).Ln().Flush()
		return
	}

	_ = sc.Start()
	keywords, _ := sc.Keywords()
	if keywords != "" {
		tt := bookmark.ParseTags(keywords)
		b.tags = tt
		f.Text(" " + color.Gray(b.tags).
			Italic().String()).Ln().Flush()
		return
	}

	if config.App.Force {
		b.tags = "notag"
		f.Text(" " + color.Gray(b.tags).
			Italic().String()).Ln().Flush()
		return
	}

	// prompt|take input for tags
	f.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()

	mTags, _ := db.TagsCounterFromPath(config.App.DBPath)
	b.tags = bookmark.ParseTags(t.ChooseTags(f.Border.Mid, mTags))

	f.Clear().Mid(color.BrightBlue("Tags\t:").String()).
		Text(" " + color.Gray(b.tags).String()).Ln()

	t.ClearLine(txt.CountLines(f.String()))
	f.Flush()
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(f *frame.Frame, sc *scraper.Scraper, b *bookmarkTemp) {
	const indentation int = 10
	width := terminal.MinWidth - len(f.Border.Row)

	if b.title != "" {
		t := color.Gray(txt.SplitAndAlign(b.title, width, indentation)).String()
		f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln().Flush()
		return
	}

	// scrape data
	_ = sc.Start()
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.tags, _ = sc.Keywords()

	t := color.Gray(txt.SplitAndAlign(b.title, width, indentation)).String()
	f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln()
	if b.desc != "" {
		descColor := color.Gray(txt.SplitAndAlign(b.desc, width, indentation)).String()
		f.Mid(color.BrightOrange("Desc\t: ").String()).Text(descColor).Ln()
	}

	f.Flush()
}

// fzfFormatter returns a function to format a bookmark for the FZF menu.
func fzfFormatter(m bool) func(b *bookmark.Bookmark) string {
	cs := color.DefaultColorScheme()
	switch {
	case m:
		return func(b *bookmark.Bookmark) string {
			return bookmark.Multiline(b, cs)
		}
	default:
		return func(b *bookmark.Bookmark) string {
			return bookmark.Oneline(b, cs)
		}
	}
}
