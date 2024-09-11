package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/haaag/gm/internal/editor"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/util/scraper"
	"github.com/haaag/gm/internal/util/spinner"
	"github.com/haaag/gm/pkg/slice"
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
	urlStr := editor.ExtractBlock(content, "# URL:", "# Title:")
	title := editor.ExtractBlock(content, "# Title:", "# Tags:")
	tags := editor.ExtractBlock(content, "# Tags:", "# Description:")
	desc := editor.ExtractBlock(content, "# Description:", "# end")

	return New(urlStr, title, ParseTags(tags), desc)
}

// ScrapeAndUpdate updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func ScrapeAndUpdate(b *Bookmark) *Bookmark {
	if b.Title == "" || b.Desc == "" {
		mesg := color.Yellow("Scraping webpage...").String()
		s := spinner.New(spinner.WithMesg(mesg))
		s.Start()

		sc := scraper.New(b.URL)
		_ = sc.Scrape()

		s.Stop()

		b.Title = validateAttr(b.Title, sc.GetTitle())
		b.Desc = validateAttr(b.Desc, sc.GetDesc())
	}

	return b
}

// BufferValidate checks if the URL and Tags are in the content.
func BufferValidate(b *[]string) error {
	if err := validateURLBuffer(b); err != nil {
		return err
	}

	return validateTagsBuffer(b)
}

// validateURLBuffer validates url in the buffer.
func validateURLBuffer(content *[]string) error {
	u := editor.ExtractBlock(content, "# URL:", "# Title:")
	if editor.IsEmptyLine(u) {
		return fmt.Errorf("%w: URL", editor.ErrLineNotFound)
	}

	return nil
}

// validateTagsBuffer validates tags in the buffer.
func validateTagsBuffer(content *[]string) error {
	t := editor.ExtractBlock(content, "# Tags:", "# Description:")
	if editor.IsEmptyLine(t) {
		return fmt.Errorf("%w: Tags", editor.ErrLineNotFound)
	}

	return nil
}

// validateAttr validates bookmark attribute.
func validateAttr(s, fallback string) string {
	s = format.NormalizeSpace(s)
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
		return ErrURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("bookmark is invalid. Tags are empty")
		return ErrTagsEmpty
	}

	log.Print("bookmark is valid")

	return nil
}

// GetBufferSlice returns a buffer with the provided slice of bookmarks.
func GetBufferSlice(bs *slice.Slice[Bookmark]) []byte {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("## Remove the <URL> line to ignore bookmark\n")
	fmt.Fprintf(buf, "## Showing %d bookmark/s\n\n", bs.Len())
	bs.ForEach(func(b Bookmark) {
		buf.Write(formatBufferSimple(&b))
	})

	return bytes.TrimSpace(buf.Bytes())
}

// ParseTags normalizes a string of tags by separating them by commas and
// ensuring that the final string ends with a comma.
//
// from: "tag1, tag2, tag3 tag"
// to: "tag1,tag2,tag3,tag,"
func ParseTags(tags string) string {
	if tags == "" {
		return "notag"
	}
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// PrettifyTags returns a prettified tags.
func PrettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", format.MidBulletPoint)
	return strings.TrimRight(t, format.MidBulletPoint)
}

// PrettifyURLPath returns a prettified URL.
func PrettifyURLPath(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return color.Text(bURL).Bold().String()
	}

	host := color.Text(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := color.Gray(
		format.PathSmallSegment,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.PathSmallSegment)),
	)

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// PrettifyURL returns a prettified URL.
func PrettifyURL(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return color.Text(bURL).Bold().String()
	}

	host := color.Text(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := color.Gray(
		format.PathSmallSegment,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.PathSmallSegment)),
	).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}
