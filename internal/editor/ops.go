package editor

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var (
	ErrLineNotFound    = errors.New("line not found")
	ErrTagsEmpty       = errors.New("tags cannot be empty")
	ErrURLEmpty        = errors.New("URL cannot be empty")
	ErrBufferUnchanged = errors.New("buffer unchanged")
)

// BufferBuilder holds information about a bookmark edit operation.
type BufferBuilder struct {
	Item   *bookmark.Bookmark
	Header []byte
	Body   []byte
	Footer []byte
	Idx    int
	Total  int
}

func NewBufferBuilder(b *bookmark.Bookmark) *BufferBuilder {
	return &BufferBuilder{
		Item: b,
		Body: b.Buffer(),
	}
}

func (be *BufferBuilder) Buffer() []byte {
	buf := make([]byte, 0, len(be.Header)+len(be.Body)+len(be.Footer))
	buf = append(buf, be.Header...)
	buf = append(buf, be.Body...)
	buf = append(buf, be.Footer...)

	return buf
}

func bookmarkFromBytes(buf []byte) *bookmark.Bookmark {
	lines := strings.Split(string(buf), "\n") // bytes to lines
	b := bookmark.New()
	b.URL = cleanLines(txt.ExtractBlock(lines, "# URL:", "# Title:"))
	b.Title = cleanLines(txt.ExtractBlock(lines, "# Title:", "# Tags:"))
	b.Tags = bookmark.ParseTags(cleanLines(txt.ExtractBlock(lines, "# Tags:", "# Description:")))
	b.Desc = cleanLines(txt.ExtractBlock(lines, "# Description:", "# end"))

	return b
}

// cleanLines sanitazes a string by removing empty lines.
func cleanLines(s string) string {
	stringSplit := strings.Split(s, "\n")
	if len(stringSplit) == 1 {
		return s
	}

	result := make([]string, 0)

	for _, ss := range stringSplit {
		trimmed := strings.TrimSpace(ss)

		if ss == "" {
			continue
		}

		result = append(result, trimmed)
	}

	return strings.Join(result, "\n")
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
