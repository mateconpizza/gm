package bookmark

import (
	"fmt"
	"log"
	"strings"

	"github.com/gocolly/colly"
)

const (
	DefaultTitle = "Untitled (Unfiled)"
	DefaultDesc  = "No description available (Unfiled)"
	// FIX: remove replaceAll
	RedditBaseURL = "old.reddit.com"
)

type Scraper struct {
	collector *colly.Collector
	url       string
}

func NewScraper(url string) *Scraper {
	collector := colly.NewCollector()
	return &Scraper{
		url:       strings.ReplaceAll(url, "www.reddit.com", RedditBaseURL),
		collector: collector,
	}
}

func (s *Scraper) Title() (title string, err error) {
	for _, selector := range []string{
		"title",
		"meta[name=title]",
		"meta[property=title]",
		"meta[name=og:title]",
		"meta[property=og:title]",
	} {
		s.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			title = strings.TrimSpace(e.Text)
		})
	}

	err = s.collector.Visit(s.url)
	if err != nil {
		return DefaultTitle, fmt.Errorf("%w: visiting and scraping URL", err)
	}

	if title == "" {
		return DefaultTitle, nil
	}

	return title, nil
}

func (s *Scraper) Description() (desc string, err error) {
	for _, selector := range []string{
		"meta[name=description]",
		"meta[property=description]",
		"meta[name=og:description]",
		"meta[property=og:description]",
	} {
		s.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			desc = e.Attr("content")
		})
	}

	err = s.collector.Visit(s.url)
	if err != nil {
		return DefaultDesc, fmt.Errorf("%w: visiting and scraping URL", err)
	}

	if desc == "" {
		return DefaultDesc, nil
	}

	return desc, nil
}

// FetchTitleAndDescription fetches the title and/or description of the bookmark
func FetchTitleAndDescription(b *Bookmark) {
	sc := NewScraper(b.URL)

	if b.Title == DefaultTitle || b.Title == "" {
		title, err := sc.Title()
		if err != nil {
			log.Printf("Error on %s: %s\n", b.URL, err)
		}
		b.Title = title
	}

	if b.Desc == DefaultDesc || b.Desc == "" {
		description, err := sc.Description()
		if err != nil {
			log.Printf("Error on %s: %s\n", b.URL, err)
		}
		b.Desc = description
	}
}
