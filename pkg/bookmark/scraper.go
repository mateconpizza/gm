package bookmark

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/gocolly/colly"
	"golang.org/x/sync/semaphore"
)

// TODO:
// - [X] If the URL doesn't exist, return default title and description
// - [ ] Remove global `URLNotFound` `URLWithError`

const (
	BookmarkDefaultTitle string = "Untitled (Unfiled)"
	BookmarkDefaultDesc  string = "No description available (Unfiled)"
)

type Response struct {
	URL        string
	ID         int
	statusCode int
	exists     bool
	hasError   bool
}

func (r *Response) String() string {
	if config.Term.Color {
		return prettyPrintURLStatus(r.statusCode, r.ID, r.URL)
	}

	return simplePrintURLStatus(r.statusCode, r.ID, r.URL)
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

func prettifyNotFound(res *[]Response) (resLen int, msg string) {
	var notFound []Response

	for _, r := range *res {
		if r.statusCode == http.StatusNotFound {
			notFound = append(notFound, r)
		}
	}

	if len(notFound) == 0 {
		return 0, ""
	}

	notFoundLenStr := format.Text(strconv.Itoa(len(notFound))).Bold().Red()
	notFoundCode := format.Text("404").Bold().Red()

	fmt.Printf("\n%s err detail:\n", notFoundLenStr)
	for _, r := range notFound {
		fmt.Println(r.String())
	}

	return len(notFound), fmt.Sprintf("\n  + %s urls return %s code", notFoundLenStr, notFoundCode)
}

func prettifyWithError(res *[]Response) (resLen int, msg string) {
	var withError []Response

	for _, r := range *res {
		if r.hasError && r.statusCode != http.StatusNotFound {
			withError = append(withError, r)
		}
	}

	if len(withError) == 0 {
		return 0, ""
	}

	withErrorLen := format.Text(strconv.Itoa(len(withError))).Yellow().Bold()
	withNoErrorCode := format.Text("200").Green().Bold()

	fmt.Printf("\n%s warns detail:\n", withErrorLen)
	for _, r := range withError {
		fmt.Println(r.String())
	}

	return len(withError), fmt.Sprintf("\n  + %s urls did not return a %s code", withErrorLen, withNoErrorCode)
}

func prettyPrintStatus(res []Response, duration time.Duration) {
	final := fmt.Sprintf("\n> %d urls were checked\n", len(res))

	took := fmt.Sprintf("%.2fs\n", duration.Seconds())

	withErrLen, withErrStr := prettifyWithError(&res)
	notFoundLen, notFoundStr := prettifyNotFound(&res)

	withNoError := strconv.Itoa(len(res) - notFoundLen - withErrLen)
	final += fmt.Sprintf(
		"  + %s urls return %s code",
		format.Text(withNoError).Blue().Bold().String(),
		format.Text("200").Green().Bold(),
	)

	final += withErrStr
	final += notFoundStr

	final += fmt.Sprintf("\n  + it took %s\n", format.Text(took).Blue().Bold().String())
	fmt.Print(final)
}

func CheckStatus(bs *[]Bookmark) error {
	maxConRequests := 50
	sem := semaphore.NewWeighted(int64(maxConRequests))

	var responses []Response
	var wg sync.WaitGroup
	start := time.Now()

	for _, b := range *bs {
		tempB := b
		wg.Add(1)

		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}

		time.Sleep(50 * time.Millisecond)
		go func(b *Bookmark) {
			defer wg.Done()
			res := makeRequest(b, sem)
			responses = append(responses, res)
		}(&tempB)
	}

	wg.Wait()

	duration := time.Since(start)
	prettyPrintStatus(responses, duration)

	return nil
}

func makeRequest(b *Bookmark, sem *semaphore.Weighted) Response {
	defer sem.Release(1)

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		fmt.Printf("Error creating request for %s: %v\n", b.URL, err)
		return Response{
			URL:        b.URL,
			ID:         b.ID,
			statusCode: 404,
			exists:     false,
			hasError:   true,
		}
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request to %s: %v\n", b.URL, err)
		return Response{
			URL:        b.URL,
			ID:         b.ID,
			statusCode: 404,
			exists:     false,
			hasError:   true,
		}
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}()

	result := Response{
		URL:        b.URL,
		ID:         b.ID,
		statusCode: resp.StatusCode,
		exists:     true,
		hasError:   resp.StatusCode != http.StatusOK,
	}

	fmt.Println(result.String())
	return result
}

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
