package handler

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/scraper"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// cleanDuplicates removes duplicate bookmarks from the import process.
func cleanDuplicates(r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
		_, exists := r.Has(b.URL)
		return !exists
	})
	if originalLen != bs.Len() {
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Flush()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// ReadURLFromClipboard checks if there a valid URL in the clipboard.
func ReadURLFromClipboard(f *frame.Frame, t *terminal.Term) string {
	c := sys.ReadClipboard()
	if !URLValid(c) {
		return ""
	}

	f.Mid(color.BrightMagenta("URL\t:").String()).
		Textln(" " + color.Gray(c).String())

	lines := format.CountLines(f.String())
	f.Flush()

	if err := t.ConfirmErr(f.Clear().Question("found valid URL in clipboard, use URL?").String(), "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return c
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(f *frame.Frame, t *terminal.Term, args *[]string) (string, error) {
	// checks if url is provided
	if len(*args) > 0 {
		bURL := strings.TrimRight((*args)[0], "\n")
		f.Header(color.BrightMagenta("URL\t:").String()).
			Text(" " + color.Gray(bURL).String()).Ln().Flush()
		*args = (*args)[1:]

		return bURL, nil
	}

	// checks clipboard
	c := ReadURLFromClipboard(f.Clear(), t)
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
func tagsFromArgs(f *frame.Frame, t *terminal.Term, tagsFlag string) string {
	f.Header(color.BrightBlue("Tags\t:").String()).Flush()
	if tagsFlag != "" {
		tagsFlag = bookmark.ParseTags(tagsFlag)
		f.Text(" " + color.Gray(tagsFlag).String()).Ln().Flush()
		return tagsFlag
	}

	// prompt for tags
	f.Text(color.Gray(" (spaces|comma separated)").Italic().String()).Ln().Flush()

	mTags, _ := repo.TagsCounterFromPath(config.App.DBPath)
	tags := bookmark.ParseTags(t.ChooseTags(f.Border.Mid, mTags))

	f.Clear().Mid(color.BrightBlue("Tags\t:").String()).
		Text(" " + color.Gray(tags).String()).Ln()

	t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return bookmark.ParseTags(tags)
}

// NewBookmark fetch metadata and parses the new bookmark.
func NewBookmark(
	f *frame.Frame,
	t *terminal.Term,
	r *repo.SQLiteRepository,
	b *bookmark.Bookmark,
	title, tags string,
	args []string,
) error {
	// retrieve newURL
	newURL, err := newURLFromArgs(f, t, &args)
	if err != nil {
		return err
	}
	newURL = strings.TrimRight(newURL, "/")
	if b, exists := r.Has(newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrDuplicate, b.ID)
	}

	// retrieve tags
	tags = tagsFromArgs(f.Clear(), t, tags)

	// fetch title and description
	var desc string
	title, desc = parseTitleAndDescription(f, newURL, title)
	b.Desc = strings.Join(format.SplitIntoChunks(desc, terminal.MinWidth), "\n")

	b.URL = newURL
	b.Title = title
	b.Tags = tags

	return nil
}

// parseTitleAndDescription fetch and display title and description.
func parseTitleAndDescription(f *frame.Frame, bURL, titleFlag string) (title, desc string) {
	const indentation int = 10
	width := terminal.MinWidth - len(f.Border.Row)

	if titleFlag != "" {
		t := color.Gray(format.SplitAndAlign(titleFlag, width, indentation)).String()
		f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln().Flush()
		return titleFlag, desc
	}

	sp := rotato.New(
		rotato.WithMesg("scraping webpage..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithSpinnerColor(rotato.ColorBrightMagenta),
	)
	sp.Start()

	// scrape data
	sc := scraper.New(bURL)
	if err := sc.Scrape(); err != nil {
		return title, desc
	}
	title = sc.Title()
	desc = sc.Desc()
	sp.Done()

	t := color.Gray(format.SplitAndAlign(title, width, indentation)).String()
	f.Mid(color.BrightCyan("Title\t: ").String()).Text(t).Ln()
	if desc != "" {
		descColor := color.Gray(format.SplitAndAlign(desc, width, indentation)).String()
		f.Mid(color.BrightOrange("Desc\t: ").String()).Text(descColor).Ln()
	}

	f.Flush()

	return title, desc
}
