package bookmark

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gomarks/pkg/app"
	"gomarks/pkg/format"

	"github.com/gocolly/colly"
	"golang.org/x/sync/semaphore"
)

// TODO:
// - [X] If the URL doesn't exist, return default title and description
// - [ ] Remove global `URLNotFound` `URLWithError`

const (
	DefaultTitle string = "Untitled (Unfiled)"
	DefaultDesc  string = "No description available (Unfiled)"
)

var URLNotFound []Response
var URLWithError []Response

type Response struct {
	URL        string
	ID         int
	StatusCode int
	Exists     bool
}

func (r *Response) String() string {
	if app.Term.Color {
		return prettyPrintURLStatus(r.StatusCode, r.ID, r.URL)
	}

	return simplePrintURLStatus(r.StatusCode, r.ID, r.URL)
}

func simplePrintURLStatus(statusCode, bID int, url string) string {
	var status string

	switch statusCode {
	case http.StatusNotFound:
		status = "ER"
	case http.StatusOK:
		status = "OK"
	default:
		status = "WA"
	}

	return fmt.Sprintf(":ID:[%-4d]:Code:[%d]:Status:[%s]:URL:%s", bID, statusCode, status, url)
}

func prettyPrintURLStatus(statusCode, bID int, url string) string {
	var colorStatus, colorCode string

	switch statusCode {
	case http.StatusNotFound:
		colorStatus = format.Text("ER").Red().Bold().String()
		colorCode = format.Text(strconv.Itoa(statusCode)).Red().Bold().String()
	case http.StatusOK:
		colorStatus = format.Text("OK").Green().Bold().String()
		colorCode = format.Text(strconv.Itoa(statusCode)).Green().Bold().String()
	default:
		colorStatus = format.Text("WA").Yellow().Bold().String()
		colorCode = format.Text(strconv.Itoa(statusCode)).Yellow().Bold().String()
	}

	idStr := format.Text(fmt.Sprintf("%-3d", bID)).Purple().Bold()
	id := format.Text(":ID:").Gray().String()
	id += fmt.Sprintf("[%s]", idStr)

	code := format.Text(":Code:").Gray().String()
	code += fmt.Sprintf("[%s]", colorCode)

	status := format.Text(":Status:").Gray().String()
	status += fmt.Sprintf("[%s]", colorStatus)

	u := format.Text(":URL:").Gray().String()
	u += format.ShortenString(url, 80)

	return fmt.Sprintf("%s%s%s%s", id, code, status, u)
}

func prettyPrintStatus(bs *Slice, duration time.Duration) {
	var final string
	took := fmt.Sprintf("%.2fs\n", duration.Seconds())

	n := strconv.Itoa(len(*bs))
	s := format.Text(n).Blue().Bold().String()
	t := format.Text(took).Blue().Bold().String()

	withErrorColor := format.Text(strconv.Itoa(len(URLWithError))).Yellow().Bold()
	withNoErrorCode := format.Text("200").Green().Bold()

	final += fmt.Sprintf("\n> %s urls return %s code", s, withNoErrorCode)

	notFoundColor := format.Text(strconv.Itoa(len(URLNotFound))).Bold().Red()
	notFoundCode := format.Text("404").Bold().Red()

	if len(URLWithError) > 0 {
		final += fmt.Sprintf("\n> %s urls did not return a %s code", withErrorColor, withNoErrorCode)
		fmt.Printf("\n%s warns detail:\n", withErrorColor)
		for _, r := range URLWithError {
			fmt.Println(r.String())
		}
	}

	if len(URLNotFound) > 0 {
		final += fmt.Sprintf("\n> %s urls return %s code", notFoundColor, notFoundCode)
		fmt.Printf("\n%s err detail:\n", notFoundColor)
		for _, r := range URLNotFound {
			fmt.Println(r.String())
		}
	}

	final += fmt.Sprintf("\n> it took %s\n", t)
	fmt.Print(final)
}

func CheckBookmarkStatus(bs *Slice) error {
	if len(*bs) == 0 {
		return ErrBookmarkNotSelected
	}

	maxConRequests := 50
	sem := semaphore.NewWeighted(int64(maxConRequests))

	var wg sync.WaitGroup
	start := time.Now()

	for _, b := range *bs {
		tempB := b
		wg.Add(1)

		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}

		go func(b *Bookmark) {
			defer wg.Done()
			makeRequest(b, sem)
		}(&tempB)
	}

	wg.Wait()

	duration := time.Since(start)
	prettyPrintStatus(bs, duration)

	return nil
}

func makeRequest(b *Bookmark, sem *semaphore.Weighted) {
	defer sem.Release(1)

	timeout := 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		fmt.Printf("Error creating request for %s: %v\n", b.URL, err)
		return
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request to %s: %v\n", b.URL, err)
		return
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		URLNotFound = append(URLNotFound, Response{URL: b.URL, ID: b.ID, Exists: false, StatusCode: resp.StatusCode})
	} else if resp.StatusCode != http.StatusOK {
		URLWithError = append(URLWithError, Response{URL: b.URL, ID: b.ID, Exists: false, StatusCode: resp.StatusCode})
	}

	result := Response{
		URL:        b.URL,
		ID:         b.ID,
		StatusCode: resp.StatusCode,
		Exists:     true,
	}

	fmt.Println(result.String())
}

func fetchTitle(url string) (string, error) {
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

func fetchDescription(url string) (string, error) {
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
