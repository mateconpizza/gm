package scraper

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultTitle string = "untitled (unfiled)"
	defaultDesc  string = "no description available (unfiled)"
)

type Scraper struct {
	doc *goquery.Document
	uri string
}

// Scrape fetches and parses the URL content.
func (s *Scraper) Scrape() error {
	s.doc = scrapeURL(s.uri)
	return nil
}

// Title retrieves the page title from the Scraper's Doc field, falling back
// to a default value if not found.
//
// default: `untitled (unfiled)`
func (s *Scraper) Title() string {
	t := s.doc.Find("title").Text()
	if t == "" {
		return defaultTitle
	}

	return strings.TrimSpace(t)
}

// Desc retrieves the page description from the Scraper's Doc field,
// defaulting to a predefined value if not found.
//
// default: `no description available (unfiled)`
func (s *Scraper) Desc() string {
	var desc string
	for _, selector := range []string{
		"meta[name=description]",
		"meta[property=description]",
	} {
		desc = s.doc.Find(selector).AttrOr("content", "")
		if desc != "" {
			break
		}
	}

	if desc == "" {
		return defaultDesc
	}

	return strings.TrimSpace(desc)
}

// New creates a new Scraper.
func New(s string) *Scraper {
	return &Scraper{uri: s}
}

// scrapeURL fetches and parses the HTML content from a URL.
func scrapeURL(s string) *goquery.Document {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s, http.NoBody)
	if err != nil {
		log.Printf("error creating request: %v", err)
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(""))

		return doc
	}

	// Set a User-Agent header
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0",
	)

	// Create a new HTTP client
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("error doing request: %v", err)
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(""))

		return doc
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

	// Parse the HTML response body
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("error creating document: %v", err)
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(""))

		return doc
	}

	return doc
}
