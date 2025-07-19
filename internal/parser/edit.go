package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var ErrBufferUnchanged = errors.New("buffer unchanged")

// BookmarkEdit holds information about a bookmark edit operation.
type BookmarkEdit struct {
	item   *bookmark.Bookmark
	header []byte
	body   []byte
	footer []byte
	idx    int
	total  int
}

func newBookmarkEdit(b *bookmark.Bookmark) *BookmarkEdit {
	return &BookmarkEdit{
		item: b,
		body: b.Buffer(),
	}
}

func (be *BookmarkEdit) Buffer() []byte {
	buf := make([]byte, 0, len(be.header)+len(be.body)+len(be.footer))
	buf = append(buf, be.header...)
	buf = append(buf, be.body...)
	buf = append(buf, be.footer...)

	return buf
}

// Edit edits a bookmark and validates the resulting content.
func Edit(te *files.TextEditor, b *bookmark.Bookmark, idx, total int) (*bookmark.Bookmark, error) {
	be := newBookmarkEdit(b)
	be.idx = idx
	be.total = total

	original := bytes.Clone(be.body)

	prepareBufferForEdition(be)

	modifiedData, err := te.EditBytes(be.Buffer(), config.App.Name)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if bytes.Equal(modifiedData, original) {
		return nil, ErrBufferUnchanged
	}

	lines := strings.Split(string(modifiedData), "\n") // bytes to lines
	if err := ValidateBookmarkFormat(lines); err != nil {
		return nil, fmt.Errorf("invalid bookmark format: %w", err)
	}

	tb := BookmarkContent(lines)
	if be.item.Equals(tb) {
		return nil, ErrBufferUnchanged
	}

	tb = scrapeBookmark(tb)
	tb.ID = be.item.ID
	tb.CreatedAt = be.item.CreatedAt
	tb.Favorite = be.item.Favorite
	tb.LastVisit = be.item.LastVisit
	tb.VisitCount = be.item.VisitCount
	tb.FaviconURL = be.item.FaviconURL

	return tb, nil
}

// prepareBufferForEdition prepares the buffer for edition.
func prepareBufferForEdition(be *BookmarkEdit) {
	const spaces = 10

	newBookmark := be.item.ID == 0

	// header
	shortTitle := txt.Shorten(be.item.Title, terminal.MinWidth-spaces-6)

	header := fmt.Appendf(nil, "# %d %s\n#\n", be.item.ID, shortTitle)
	if newBookmark {
		header = fmt.Appendf(nil, "# %s\n#\n", shortTitle)
	}

	// header mesg
	s := "bookmark edition"
	if newBookmark {
		s = "bookmark addition"
	}

	sep := txt.CenteredLine(terminal.MinWidth-spaces, s)

	// metadata
	meta := fmt.Appendf(nil,
		"# database:\t%q\n# version:\tv%s\n# %s\n\n",
		config.App.DBName,
		config.App.Info.Version,
		sep,
	)

	// footer
	be.footer = fmt.Appendf(nil, " [%d/%d]", be.idx+1, be.total)
	if newBookmark {
		be.footer = fmt.Appendf(nil, " [New]")
	}

	// assemble
	header = append(header, meta...)
	be.header = append(be.header, header...)
}

// scrapeBookmark updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func scrapeBookmark(b *bookmark.Bookmark) *bookmark.Bookmark {
	if b.Title != "" {
		return b
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sc := scraper.New(b.URL, scraper.WithContext(ctx), scraper.WithSpinner("scraping webpage..."))
	if err := sc.Start(); err != nil {
		slog.Error("scraping error", "error", err)
	}

	if b.Title == "" {
		t, _ := sc.Title()
		b.Title = validateAttr(b.Title, t)
	}

	if b.Desc == "" {
		d, _ := sc.Desc()
		b.Desc = validateAttr(b.Desc, d)
	}

	f, _ := sc.Favicon()
	b.FaviconURL = f

	return b
}

// validateAttr validates bookmark attribute.
func validateAttr(s, fallback string) string {
	s = strings.TrimSpace(txt.NormalizeSpace(s))
	if s == "" {
		return strings.TrimSpace(fallback)
	}

	return s
}

// Tags normalizes a string of tags by separating them by commas, sorting
// them and ensuring that the final string ends with a comma.
//
//	from: "tag1, tag2, tag3 tag"
//	to: "tag,tag1,tag2,tag3,"
func Tags(tags string) string {
	if tags == "" {
		return "notag"
	}

	split := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	})
	sort.Strings(split)

	tags = strings.Join(uniqueTags(split), ",")
	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// uniqueTags returns a slice of unique tags.
func uniqueTags(t []string) []string {
	var (
		tags []string
		seen = make(map[string]bool)
	)

	for _, tag := range t {
		if tag == "" {
			continue
		}

		if !seen[tag] {
			seen[tag] = true

			tags = append(tags, tag)
		}
	}

	return tags
}
