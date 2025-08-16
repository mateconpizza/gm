package status

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

var (
	cb  = func(s any) string { return color.Blue(s).Bold().String() }
	cbg = func(s any) string { return color.BrightGreen(s).String() }
	cr  = func(s any) string { return color.Red(s).String() }
	cbr = func(s any) string { return color.BrightRed(s).String() }
	cy  = func(s any) string { return color.Yellow(s).String() }
	ctb = func(s string) string { return color.Text(s).Bold().String() }
)

type Response struct {
	URL        string
	bID        int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	id := fmt.Sprintf("ID %s", color.Text(fmt.Sprintf("%-3d", r.bID)).Bold())
	colorStatus, colorCode := prettifyURLStatus(r.statusCode)
	url := txt.Shorten(r.URL, terminal.MinWidth)

	return fmt.Sprintf("%s (%s %s) %s", id, colorCode, colorStatus, url)
}

// Check checks the status of a slice of bookmarks.
func Check(c *ui.Console, bs []*bookmark.Bookmark) error {
	const maxConRequests = 25

	var (
		responses = make([]*Response, 0, len(bs))
		sem       *semaphore.Weighted
		start     time.Time
		wg        sync.WaitGroup
		mu        sync.Mutex
	)

	sem = semaphore.NewWeighted(int64(maxConRequests))
	start = time.Now()
	ctx := context.Background()

	schedule := func(b *bookmark.Bookmark) error {
		wg.Add(1)

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}

		time.Sleep(50 * time.Millisecond)

		go func(b *bookmark.Bookmark) {
			defer wg.Done()

			res := makeRequest(c, b, ctx, sem)
			mu.Lock()
			responses = append(responses, &res)
			mu.Unlock()
		}(b)

		return nil
	}

	for _, b := range bs {
		if err := schedule(b); err != nil {
			return err
		}
	}

	wg.Wait()

	duration := time.Since(start)
	printSummaryStatus(c, responses, duration)

	return nil
}

// prettifyURLStatus formats HTTP status codes into colored.
func prettifyURLStatus(code int) (status, statusCode string) {
	statusCategory := code / 100

	switch statusCategory {
	case 2: // 2xx status codes
		status = cbg("OK")
		statusCode = cbg(code)
	case 3: // 3xx status codes
		status = cy("WA")
		statusCode = cy(code)
	case 4: // 4xx status codes
		status = cbr("ER")
		statusCode = cbr(code)
	case 5: // 5xx status codes
		status = cr("ER")
		statusCode = cr(code)
	default: // Other status codes
		status = cy("WA")
		statusCode = cy(code)
	}
	return status, statusCode
}

// fmtSummary formats the summary of the status codes.
func fmtSummary(n, statusCode int, c color.ColorFn) string {
	total := fmt.Sprintf(c("%-3d").Bold().String(), n)
	code := c(statusCode).String()
	s := http.StatusText(statusCode)

	statusText := color.Text(s).Italic().String()
	if s == "" {
		statusText = color.Text("non-standard code").Italic().String()
	}

	return total + " URLs returned '" + statusText + "' (" + code + ")"
}

// printSummaryStatus prints a summary of HTTP status codes and their
// corresponding URLs.
func printSummaryStatus(c *ui.Console, r []*Response, d time.Duration) {
	codes := make(map[int][]Response)

	c.F.Rowln().Header(ctb("Summary URLs status:\n"))

	for _, res := range r {
		codes[res.statusCode] = append(codes[res.statusCode], *res)
	}

	for statusCode, res := range codes {
		n := len(res)

		statusCategory := statusCode / 100
		switch statusCategory {
		case 2: // 2xx status codes
			c.F.Midln(fmtSummary(n, statusCode, color.BrightGreen))
		case 3: // 3xx status codes
			c.F.Midln(fmtSummary(n, statusCode, color.Yellow))
		case 4: // 4xx status codes
			c.F.Midln(fmtSummary(n, statusCode, color.BrightRed))
		case 5: // 5xx status codes
			c.F.Midln(fmtSummary(n, statusCode, color.Red))
		default: // Other status codes
			c.F.Midln(fmtSummary(n, statusCode, color.Yellow))
		}

		// adds URLs detail
		for _, r := range res {
			// ignore 200 response
			if r.statusCode == http.StatusOK {
				continue
			}
			c.F.Rowln(fmt.Sprintf(" > %-3d %s", r.bID, txt.Shorten(r.URL, terminal.MinWidth)))
		}
	}

	took := fmt.Sprintf("%.2fs", d.Seconds())
	total := fmt.Sprintf("Total %s checked,", cb(len(r)))
	c.F.Rowln().Footerln(total + " took " + cb(took)).Flush()
}

// buildResponse builds a Response from an HTTP response.
func buildResponse(c *ui.Console, b *bookmark.Bookmark, statusCode int, hasError bool) Response {
	result := Response{
		URL:        b.URL,
		bID:        b.ID,
		statusCode: statusCode,
		hasError:   hasError,
	}

	b.HTTPStatusCode = statusCode
	b.HTTPStatusText = http.StatusText(statusCode)
	b.IsActive = statusCode >= 200 && statusCode <= 299
	b.LastStatusChecked = time.Now().Format("20060102150405")

	statusCategory := statusCode / 100
	switch statusCategory {
	case 2: // 2xx status codes
		c.F.Success(result.String() + "\n").Flush()
	case 3: // 3xx status codes
		c.F.Warning(result.String() + "\n").Flush()
	case 4: // 4xx status codes
		c.F.Error(result.String() + "\n").Flush()
	case 5: // 5xx status codes
		c.F.Error(result.String() + "\n").Flush()
	default: // Other status codes
		c.F.Midln(result.String()).Flush()
	}

	return result
}

// handleRequestError handles errors from the HTTP request and determines the
// appropriate status code.
func handleRequestError(c *ui.Console, b *bookmark.Bookmark, err error) Response {
	var statusCode int

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		statusCode = http.StatusGatewayTimeout
	case isNetworkUnreachableError(err):
		statusCode = http.StatusServiceUnavailable
	case errors.Is(err, context.Canceled):
		statusCode = http.StatusNotFound
	default:
		statusCode = http.StatusNotFound
	}

	return buildResponse(c, b, statusCode, true)
}

// makeRequest sends an HTTP GET request to the URL of the given bookmark and
// returns a response.
//
// The function uses a weighted semaphore to limit the number of concurrent
// requests.
func makeRequest(c *ui.Console, b *bookmark.Bookmark, ctx context.Context, sem *semaphore.Weighted) Response {
	// FIX: Split this function
	defer sem.Release(1)

	timeout := 5 * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		slog.Error("creating request", slog.String("url", b.URL), slog.String("error", err.Error()))
		return buildResponse(c, b, http.StatusNotFound, true)
	}

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		return handleRequestError(c, b, err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("error closing response body:", err)
		}
	}()

	return buildResponse(c, b, resp.StatusCode, resp.StatusCode != http.StatusOK)
}

func isNetworkUnreachableError(err error) bool {
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		return netOpErr.Op == "connect" &&
			strings.Contains(netOpErr.Err.Error(), "network is unreachable")
	}

	return false
}
