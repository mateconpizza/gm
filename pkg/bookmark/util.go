package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/pkg/util/scraper"
	"github.com/haaag/gm/pkg/util/spinner"
)

// ExtractIDs extracts the IDs from a slice of bookmarks.
func ExtractIDs(bs *[]Bookmark) []int {
	ids := make([]int, 0, len(*bs))
	for _, b := range *bs {
		ids = append(ids, b.ID)
	}

	return ids
}

// ParseContent parses the provided content into a bookmark struct.
func ParseContent(content *[]string) *Bookmark {
	url := editor.ExtractBlock(content, "# URL:", "# Title:")
	title := editor.ExtractBlock(content, "# Title:", "# Tags:")
	tags := editor.ExtractBlock(content, "# Tags:", "# Description:")
	desc := editor.ExtractBlock(content, "# Description:", "# end")
	b := New(url, title, format.ParseTags(tags), desc)

	if b.Title == "" || b.Desc == "" {
		s := spinner.New()
		s.Mesg = "Scraping webpage..."
		s.Start()

		sc := scraper.New(b.URL)
		_ = sc.Scrape()

		s.Stop()

		b.Title = ValidateAttr(b.Title, sc.GetTitle())
		b.Desc = ValidateAttr(b.Desc, sc.GetDesc())
	}

	return b
}

// normalizeSpace removes extra whitespace from a string, leaving only single
// spaces between words.
func normalizeSpace(s string) string {
	s = strings.TrimSpace(s)
	return strings.Join(strings.Fields(s), " ")
}

// ValidateAttr validates bookmark attribute.
func ValidateAttr(s, fallback string) string {
	s = normalizeSpace(s)
	s = strings.TrimSpace(s)

	if s == "" {
		return strings.TrimSpace(fallback)
	}

	return s
}

// Validate validates the bookmark.
func Validate(b *Bookmark) error {
	if b.URL == "" {
		log.Print("bookmark is invalid. URL is empty")
		return ErrBookmarkURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("bookmark is invalid. Tags are empty")
		return ErrBookmarkTagsEmpty
	}

	log.Print("bookmark is valid")

	return nil
}

// GetBufferSlice returns a buffer with the provided slice of bookmarks.
func GetBufferSlice(bs *slice.Slice[Bookmark]) []byte {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("## Remove the <URL> line to ignore\n")
	fmt.Fprintf(buf, "## Showing %d bookmark/s\n\n", bs.Len())
	bs.ForEach(func(b Bookmark) {
		buf.Write(b.BufSimple())
	})

	return bytes.TrimSpace(buf.Bytes())
}
