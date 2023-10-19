package scrape

import (
	"strings"

	"github.com/gocolly/colly"
)

type ScrapeResult struct {
	Title       string
	Description string
}

func TitleAndDescription(url string) (*ScrapeResult, error) {
	url = strings.Replace(url, "www.reddit.com", "old.reddit.com", -1)

	c := colly.NewCollector()

	result := &ScrapeResult{}

	c.OnHTML("title", func(e *colly.HTMLElement) {
		result.Title = strings.TrimSpace(e.Text)
	})

	c.OnHTML("meta[name=description]", func(e *colly.HTMLElement) {
		result.Description = e.Attr("content")
	})

	err := c.Visit(url)
	if err != nil {
		return nil, err
	}

	return result, nil
}
