package bookmark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark/scraper"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var ErrLineNotFound = errors.New("line not found")

// ParseTags normalizes a string of tags by separating them by commas, sorting
// them and ensuring that the final string ends with a comma.
//
//	from: "tag1, tag2, tag3 tag"
//	to: "tag,tag1,tag2,tag3,"
func ParseTags(tags string) string {
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

// Validate validates the bookmark.
func Validate(b *Bookmark) error {
	if b.URL == "" {
		slog.Error("bookmark is invalid. URL is empty")
		return ErrURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		slog.Error("bookmark is invalid. Tags are empty")
		return ErrTagsEmpty
	}

	return nil
}

// ScrapeMissingDescription scrapes missing data from bookmarks found from the import
// process.
func ScrapeMissingDescription(bs []*Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	sp := rotato.New(
		rotato.WithSpinnerColor(rotato.ColorGray),
		rotato.WithMesg("scraping missing data..."),
		rotato.WithMesgColor(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
	)
	sp.Start()

	var (
		wg   sync.WaitGroup
		errs = make([]string, 0)
	)

	for _, b := range bs {
		wg.Add(1)

		go func(b *Bookmark) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer wg.Done()

			sc := scraper.New(b.URL, scraper.WithContext(ctx))

			if err := sc.Start(); err != nil {
				errs = append(errs, fmt.Sprintf("url %s: %s", b.URL, err.Error()))
				slog.Warn("scraping error", "url", b.URL, "err", err)
			}

			b.Desc, _ = sc.Desc()
		}(b)
	}

	wg.Wait()

	sp.Done("Scraping done")

	return nil
}

func ValidateChecksumJSON(b *BookmarkJSON) bool {
	tags := ParseTags(strings.Join(b.Tags, ","))
	return b.Checksum == Checksum(b.URL, b.Title, b.Desc, tags)
}

func ValidateChecksum(b *Bookmark) bool {
	return b.Checksum == Checksum(b.URL, b.Title, b.Desc, b.Tags)
}

// validateBookmarkFormat checks if the URL and Tags are in the content.
func validateBookmarkFormat(b []string) error {
	if err := validateURLBuffer(b); err != nil {
		return err
	}

	return validateTagsBuffer(b)
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

// parseBookmarkContent parses the provided content into a bookmark struct.
func parseBookmarkContent(lines []string) *Bookmark {
	b := New()
	b.URL = cleanLines(txt.ExtractBlock(lines, "# URL:", "# Title:"))
	b.Title = cleanLines(txt.ExtractBlock(lines, "# Title:", "# Tags:"))
	b.Tags = ParseTags(cleanLines(txt.ExtractBlock(lines, "# Tags:", "# Description:")))
	b.Desc = cleanLines(txt.ExtractBlock(lines, "# Description:", "# end"))

	return b
}

// validateURLBuffer validates url in the buffer.
func validateURLBuffer(content []string) error {
	u := txt.ExtractBlock(content, "# URL:", "# Title:")
	if strings.TrimSpace(u) == "" {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}

	return nil
}

// validateTagsBuffer validates tags in the buffer.
func validateTagsBuffer(content []string) error {
	t := txt.ExtractBlock(content, "# Tags:", "# Description:")
	if strings.TrimSpace(t) == "" {
		return fmt.Errorf("%w: Tags", ErrLineNotFound)
	}

	return nil
}

// validateAttr validates bookmark attribute.
func validateAttr(s, fallback string) string {
	s = strings.TrimSpace(txt.NormalizeSpace(s))
	if s == "" {
		return strings.TrimSpace(fallback)
	}

	return s
}

// scrapeBookmark updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func scrapeBookmark(b *Bookmark) *Bookmark {
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

// hashURL generates a hash from a hashURL.
func hashURL(rawURL string) string {
	return txt.GenHash(rawURL, 12)
}

// hashDomain generates a hash from a domain.
func hashDomain(rawURL string) (string, error) {
	domain, err := domain(rawURL)
	if err != nil {
		return "", err
	}

	return txt.GenHash(domain, 12), nil
}

// domain extracts the domain from a URL.
func domain(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing url: %w", err)
	}

	// normalize domain
	domain := strings.ToLower(u.Hostname())

	return strings.TrimPrefix(domain, "www."), nil
}

// Checksum generates a checksum for the bookmark.
func Checksum(rawURL, title, desc, tags string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s", rawURL, title, desc, tags)
	return txt.GenHash(data, 8)
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
