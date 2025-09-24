package parser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var (
	ErrTagsEmpty    = errors.New("tags cannot be empty")
	ErrURLEmpty     = errors.New("URL cannot be empty")
	ErrLineNotFound = errors.New("line not found")
)

// ScrapeMissingDescription scrapes missing data from bookmarks found from the import
// process.
func ScrapeMissingDescription(bs []*bookmark.Bookmark) error {
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

		go func(b *bookmark.Bookmark) {
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

func ValidateChecksumJSON(b *bookmark.BookmarkJSON) bool {
	tags := bookmark.ParseTags(strings.Join(b.Tags, ","))
	return b.Checksum == bookmark.GenChecksum(b.URL, b.Title, b.Desc, tags)
}

// BookmarkContent parses the provided content into a bookmark struct.
func BookmarkContent(lines []string) *bookmark.Bookmark {
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

// ValidateBookmarkFormat checks if the URL and Tags are in the content.
func ValidateBookmarkFormat(b []string) error {
	if err := validateURLBuffer(b); err != nil {
		return err
	}

	return validateTagsBuffer(b)
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
