package scrape

import (
	"log"
	"strings"

	"github.com/gocolly/colly"
)

type Result struct {
	Title       string
	Description string
}

func TitleAndDescription(url string) (*Result, error) {
	url = strings.ReplaceAll(url, "www.reddit.com", "old.reddit.com")

	titleSelectors := []string{
		"title",
		"meta[name=title]",
		"meta[property=title]",
		"meta[name=og:title]",
		"meta[property=og:title]",
	}
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

	result := &Result{}

	for _, selector := range titleSelectors {
		c.OnHTML(selector, func(e *colly.HTMLElement) {
			result.Title = strings.TrimSpace(e.Text)
		})

		if result.Title != "" {
			break
		}
	}

	for _, selector := range descSelectors {
		c.OnHTML(selector, func(e *colly.HTMLElement) {
			result.Description = e.Attr("content")
		})

		if result.Description != "" {
			break
		}
	}

	c.OnResponse(func(r *colly.Response) {
		log.Println("Got a response from", r.Request.URL)
	})

	err := c.Visit(url)
	if err != nil {
		return nil, err
	}

	log.Printf("Title: %s", result.Title)
	log.Printf("Description: %s", result.Description)

	return result, nil
}
