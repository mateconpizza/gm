package bookmark

import (
	"fmt"
	"log"
	"strings"

	"github.com/gocolly/colly"
)

const (
	BookmarkDefaultTitle string = "Untitled (Unfiled)"
	BookmarkDefaultDesc  string = "No description available (Unfiled)"
)

func Title(url string) (string, error) {
	url = strings.ReplaceAll(url, "www.reddit.com", "old.reddit.com")

	titleSelectors := []string{
		"title",
		"meta[name=title]",
		"meta[property=title]",
		"meta[name=og:title]",
		"meta[property=og:title]",
	}

	c := colly.NewCollector()
	var title string

	for _, selector := range titleSelectors {
		c.OnHTML(selector, func(e *colly.HTMLElement) {
			title = strings.TrimSpace(e.Text)
		})

		if title != "" {
			break
		}
	}

	err := c.Visit(url)
	if err != nil {
		return BookmarkDefaultTitle, fmt.Errorf("%w: visiting and scraping URL", err)
	}

	if title == "" {
		return BookmarkDefaultTitle, nil
	}

	return title, nil
}

func Description(url string) (string, error) {
	url = strings.ReplaceAll(url, "www.reddit.com", "old.reddit.com")

	descSelectors := []string{
		"meta[name=description]",
		"meta[name=Description]",
		"meta[property=description]",
		"meta[property=Description]",
		"meta[name=og:description]",
		"meta[name=og:Description]",
		"meta[property=og:description]",
		"meta[property=og:Description]",
	}

	c := colly.NewCollector()
	var description string

	for _, selector := range descSelectors {
		c.OnHTML(selector, func(e *colly.HTMLElement) {
			description = e.Attr("content")
		})

		if description != "" {
			break
		}
	}

	err := c.Visit(url)
	if err != nil {
		return BookmarkDefaultDesc, fmt.Errorf(
			"%w: visiting and scraping URL",
			err,
		)
	}

	if description == "" {
		return BookmarkDefaultDesc, nil
	}

	return description, nil
}

// FetchTitleAndDescription Fetches the title and/or description of the
// bookmark's URL, if they are not already set.
func FetchTitleAndDescription(b *Bookmark) {
	if b.Title == BookmarkDefaultTitle || b.Title == "" {
		title, err := Title(b.URL)
		if err != nil {
			log.Printf("Error on %s: %s\n", b.URL, err)
		}
		b.Title = title
	}

	if b.Desc == BookmarkDefaultDesc || b.Desc == "" {
		description, err := Description(b.URL)
		if err != nil {
			log.Printf("Error on %s: %s\n", b.URL, err)
		}
		b.Desc = description
	}
}
