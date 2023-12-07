package bookmark

import (
	"fmt"
	"strings"

	"github.com/gocolly/colly"
)

const (
	DefaultTitle string = "Untitled"
	DefaultDesc  string = "No description available"
)

func FetchTitle(url string) (string, error) {
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
		return DefaultTitle, fmt.Errorf("%w: visiting and scraping URL", err)
	}

	if title == "" {
		return DefaultTitle, nil
	}

	return title, nil
}

func FetchDescription(url string) (string, error) {
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
		return DefaultDesc, fmt.Errorf(
			"%w: visiting and scraping URL",
			err,
		)
	}

	if description == "" {
		return DefaultDesc, nil
	}

	return description, nil
}
