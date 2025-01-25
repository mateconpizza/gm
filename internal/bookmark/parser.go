package bookmark

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/sys/spinner"
)

var ErrLineNotFound = errors.New("line not found")

// ParseTags normalizes a string of tags by separating them by commas, sorting
// them and ensuring that the final string ends with a comma.
//
// from: "tag1, tag2, tag3 tag"
// to: "tag,tag1,tag2,tag3,"
func ParseTags(tags string) string {
	if tags == "" {
		return "notag"
	}

	split := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	})
	sort.Strings(split)
	tags = strings.Join(format.Unique(split), ",")
	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// ExtractContentLine extracts URLs from the a slice of strings.
func ExtractContentLine(c *[]string) map[string]bool {
	m := make(map[string]bool)
	for _, l := range *c {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "#") && !strings.EqualFold(l, "") {
			m[l] = true
		}
	}

	return m
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

	return nil
}

// bufferValidate checks if the URL and Tags are in the content.
func bufferValidate(b *[]string) error {
	if err := validateURLBuffer(b); err != nil {
		return err
	}

	return validateTagsBuffer(b)
}

// parseContent parses the provided content into a bookmark struct.
func parseContent(c *[]string) *Bookmark {
	b := New()
	b.URL = extractTextBlock(c, "# URL:", "# Title:")
	b.Title = extractTextBlock(c, "# Title:", "# Tags:")
	b.Tags = ParseTags(extractTextBlock(c, "# Tags:", "# Description:"))
	b.Desc = extractTextBlock(c, "# Description:", "# end")

	return b
}

// extractTextBlock extracts a block of text from a string, delimited by the
// specified start and end markers.
func extractTextBlock(content *[]string, startMarker, endMarker string) string {
	startIndex := -1
	endIndex := -1
	isInBlock := false

	var cleanedBlock []string

	for i, line := range *content {
		if strings.HasPrefix(line, startMarker) {
			startIndex = i
			isInBlock = true

			continue
		}

		if strings.HasPrefix(line, endMarker) && isInBlock {
			endIndex = i

			break // Found end marker line
		}

		if isInBlock {
			cleanedBlock = append(cleanedBlock, line)
		}
	}

	if startIndex == -1 || endIndex == -1 {
		return ""
	}

	return strings.Join(cleanedBlock, "\n")
}

// validateURLBuffer validates url in the buffer.
func validateURLBuffer(content *[]string) error {
	u := extractTextBlock(content, "# URL:", "# Title:")
	if format.IsEmptyLine(u) {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}

	return nil
}

// validateTagsBuffer validates tags in the buffer.
func validateTagsBuffer(content *[]string) error {
	t := extractTextBlock(content, "# Tags:", "# Description:")
	if format.IsEmptyLine(t) {
		return fmt.Errorf("%w: Tags", ErrLineNotFound)
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

// scrapeAndUpdate updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func scrapeAndUpdate(b *Bookmark) *Bookmark {
	if b.Title == "" || b.Desc == "" {
		mesg := color.Yellow("scraping webpage...").String()
		sp := spinner.New(spinner.WithMesg(mesg))
		sp.Start()
		defer sp.Stop()

		sc := scraper.New(b.URL)
		_ = sc.Scrape()

		b.Title = validateAttr(b.Title, sc.Title())
		b.Desc = validateAttr(b.Desc, sc.Desc())
	}

	return b
}
