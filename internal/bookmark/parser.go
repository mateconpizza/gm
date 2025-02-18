package bookmark

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/haaag/rotato"

	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/format"
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

// validateBookmarkFormat checks if the URL and Tags are in the content.
func validateBookmarkFormat(b []string) error {
	if err := validateURLBuffer(b); err != nil {
		return err
	}

	return validateTagsBuffer(b)
}

// parseBookmarkContent parses the provided content into a bookmark struct.
func parseBookmarkContent(lines []string) *Bookmark {
	b := New()
	b.URL = extractTextBlock(lines, "# URL:", "# Title:")
	b.Title = extractTextBlock(lines, "# Title:", "# Tags:")
	b.Tags = ParseTags(extractTextBlock(lines, "# Tags:", "# Description:"))
	b.Desc = extractTextBlock(lines, "# Description:", "# end")

	return b
}

// extractTextBlock extracts a block of text from a string, delimited by the
// specified start and end markers.
func extractTextBlock(content []string, startMarker, endMarker string) string {
	startIndex := -1
	endIndex := -1
	isInBlock := false

	var cleanedBlock []string

	for i, line := range content {
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
func validateURLBuffer(content []string) error {
	u := extractTextBlock(content, "# URL:", "# Title:")
	if format.IsEmptyLine(u) {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}

	return nil
}

// validateTagsBuffer validates tags in the buffer.
func validateTagsBuffer(content []string) error {
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

// scrapeBookmark updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func scrapeBookmark(b *Bookmark) *Bookmark {
	if b.Title == "" || b.Desc == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sp := rotato.New(
			rotato.WithMesg("scraping webpage..."),
			rotato.WithMesgColor(rotato.ColorYellow),
		)
		sp.Start()
		defer sp.Done()

		sc := scraper.New(b.URL, scraper.WithContext(ctx))
		if err := sc.Scrape(); err != nil {
			log.Printf("scraping error: %s", err)
		}

		b.Title = validateAttr(b.Title, sc.Title())
		b.Desc = validateAttr(b.Desc, sc.Desc())
	}

	return b
}
