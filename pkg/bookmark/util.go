package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/scraper"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/pkg/terminal"
)

const (
	_indentation = 10
)

// HandleURL handles the URL.
func HandleURL(args *[]string) string {
	urlPrompt := C("+ URL\t:").Blue().Bold().String()

	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		url = strings.TrimRight(url, "\n")
		fmt.Println(urlPrompt, url)

		return url
	}

	urlPrompt += C("\n > ").Orange().Bold().String()

	return terminal.ReadInput(urlPrompt)
}

// HandleTags handles the tags.
func HandleTags(args *[]string) string {
	tagsPrompt := C("+ Tags\t:").Purple().Bold().String()

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		tags = strings.TrimRight(tags, "\n")
		tags = strings.Join(strings.Fields(tags), ",")
		fmt.Println(tagsPrompt, tags)

		return tags
	}

	tagsPrompt += C(" (comma-separated)").Italic().Gray().String()
	tagsPrompt += C("\n > ").Orange().Bold().String()

	return terminal.ReadInput(tagsPrompt)
}

// HandleTitleAndDesc fetch and display title and description.
func HandleTitleAndDesc(url string, minWidth int) (title, desc string) {
	var r strings.Builder
	sc := scraper.New(url)
	_ = sc.Scrape()
	r.WriteString(C("+ Title\t: ").Green().Bold().String())
	r.WriteString(format.SplitAndAlignString(sc.Title, minWidth, _indentation))
	r.WriteString(C("\n+ Desc\t: ").Yellow().Bold().String())
	r.WriteString(format.SplitAndAlignString(sc.Desc, minWidth, _indentation))
	fmt.Println(r.String())

	return sc.Title, sc.Desc
}

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
	b := &Bookmark{URL: url, Title: title, Tags: format.ParseTags(tags), Desc: desc}

	if b.Title == "" || b.Desc == "" {
		sc := scraper.New(b.URL)
		_ = sc.Scrape()
		b.Title = ValidateAttr(b.Title, sc.Title)
		b.Desc = ValidateAttr(b.Desc, sc.Desc)
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
