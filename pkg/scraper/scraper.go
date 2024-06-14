package scraper

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gocolly/colly"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/util"
)

const (
	DefaultTitle = "untitled (unfiled)"
	DefaultDesc  = "no description available (unfiled)"
)

type Scraper struct {
	collector *colly.Collector
	URL       string
	Title     string
	Desc      string
}

func New(url string) *Scraper {
	collector := colly.NewCollector()
	return &Scraper{
		URL:       url,
		collector: collector,
	}
}

func getHeaders() http.Header {
	return http.Header{
		"User-Agent":      {"Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0"},
		"Accept":          {"*/*"},
		"Accept-Encoding": {"gzip, deflate"},
		"Cookie":          {""},
		"DNT":             {"1"},
	}
}

func (s *Scraper) Scrape() error {
	done := make(chan bool)
	go util.Spinner(done, format.Color("scraping title and desc...").Gray().String())

	s.collector.OnRequest(func(r *colly.Request) {
		headers := getHeaders()
		r.Headers = &headers
	})

	s.collector.OnHTML("title", func(e *colly.HTMLElement) {
		s.Title = strings.TrimSpace(e.Text)
	})

	for _, selector := range []string{
		"meta[name=description]",
		"meta[name=Description]",
		"meta[property=description]",
		"meta[property=Description]",
		"meta[name=og:description]",
		"meta[name=og:Description]",
		"meta[property=og:description]",
		"meta[property=og:Description]",
	} {
		s.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			s.Desc = e.Attr("content")
		})
	}

	if s.Title == "" {
		s.Title = DefaultTitle
	}

	if s.Desc == "" {
		s.Desc = DefaultDesc
	}

	if err := s.collector.Visit(s.URL); err != nil {
		fmt.Println(format.Color("failed to visit URL:", err.Error()).Dim().String())
	}

	done <- true
	return nil
}
