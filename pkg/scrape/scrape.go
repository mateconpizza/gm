package scrape

import (
	"log"
	"strings"

	"github.com/gocolly/colly"
)

func GetTitle(url string) (string, error) {
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

	c.OnResponse(func(r *colly.Response) {
		log.Println("Got a response from", r.Request.URL)
	})

	err := c.Visit(url)
	if err != nil {
		return "", err
	}

	return title, nil
}

func GetDescription(url string) (string, error) {
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

	c.OnResponse(func(r *colly.Response) {
		log.Println("Got a response from", r.Request.URL)
	})

	err := c.Visit(url)
	if err != nil {
		return "", err
	}

	return description, nil
}
