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
	_defaultTitle = "untitled (unfiled)"
	_defaultDesc  = "no description available (unfiled)"
)

type Scraper struct {
	Doc *goquery.Document
	URL string
}

// Scrape fetches and parses the URL content.
func (s *Scraper) Scrape() error {
	s.Doc = scrapeURL(s.URL)
	return nil
}

// GetTitle retrieves the page title from the Scraper's Doc field, falling back
// to a default value if not found.
//
// default: `untitled (unfiled)`
func (s *Scraper) GetTitle() string {
	title := s.Doc.Find("title").Text()
	if title == "" {
		return _defaultTitle
	}

	return strings.TrimSpace(title)
}

// GetDesc retrieves the page description from the Scraper's Doc field,
// defaulting to a predefined value if not found.
//
// default: `no description available (unfiled)`
func (s *Scraper) GetDesc() string {
	var desc string
	for _, selector := range []string{
		"meta[name=description]",
		"meta[property=description]",
	} {
		desc = s.Doc.Find(selector).AttrOr("content", "")
		if desc != "" {
			break
		}
	}

	if desc == "" {
		return _defaultDesc
	}

	return strings.TrimSpace(desc)
}

// New creates a new Scraper.
func New(url string) *Scraper {
	return &Scraper{URL: url}
}

// scrapeURL fetches and parses the HTML content from a URL.
func scrapeURL(url string) *goquery.Document {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
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
