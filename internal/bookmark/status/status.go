// Package status provides concurrent HTTP status checking for bookmarks.
// It performs bulk URL validation with rate limiting and colored output.
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
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

type Response struct {
	URL        string
	bID        int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	c := ui.NewConsole()
	colorStatus, colorCode := prettifyURLStatus(c, r.statusCode)

	return fmt.Sprintf(
		"ID %s (%s %s) %s",
		c.Palette().Bold(fmt.Sprintf("%-3d", r.bID)),
		colorCode,
		colorStatus,
		txt.Shorten(r.URL, terminal.MinWidth),
	)
}

// Check checks the status of a slice of bookmarks.
func Check(ctx context.Context, c *ui.Console, bs []*bookmark.Bookmark) error {
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
func prettifyURLStatus(c *ui.Console, code int) (status, statusCode string) {
	p := c.Palette()
	statusCategory := code / 100

	switch statusCategory {
	case 2: // 2xx status codes
		status = p.BrightGreen("OK")
		statusCode = p.BrightGreen(code)
	case 3: // 3xx status codes
		status = p.BrightYellow("WA")
		statusCode = p.BrightYellow(code)
	case 4: // 4xx status codes
		status = p.BrightRed("ER")
		statusCode = p.BrightRed(code)
	case 5: // 5xx status codes
		status = p.Red("ER")
		statusCode = p.Red(code)
	default: // Other status codes
		status = p.Yellow("WA")
		statusCode = p.Yellow(code)
	}
	return status, statusCode
}

// fmtSummary formats the summary of the status codes.
func fmtSummary(c *ui.Console, n, statusCode int, colorFn func(...any) string) string {
	total := fmt.Sprintf(colorFn("%-3d"), n)
	code := colorFn(statusCode)
	s := http.StatusText(statusCode)

	p := c.Palette()
	statusText := p.Italic(s)
	if s == "" {
		statusText = p.Italic("non-standard code")
	}

	return total + " URLs returned '" + statusText + "' (" + code + ")"
}

// printSummaryStatus prints a summary of HTTP status codes and their
// corresponding URLs.
func printSummaryStatus(c *ui.Console, r []*Response, d time.Duration) {
	p := c.Palette()
	codes := make(map[int][]Response)
	f := c.Frame()
	f.Rowln().Header(p.Bold("Summary URLs status:\n"))

	for _, res := range r {
		codes[res.statusCode] = append(codes[res.statusCode], *res)
	}

	for statusCode, res := range codes {
		n := len(res)

		statusCategory := statusCode / 100
		switch statusCategory {
		case 2: // 2xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.BrightGreenBold))
		case 3: // 3xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.YellowBold))
		case 4: // 4xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.BrightRedBold))
		case 5: // 5xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.RedBold))
		default: // Other status codes
			f.Midln(fmtSummary(c, n, statusCode, p.YellowBold))
		}

		// adds URLs detail
		for _, r := range res {
			// ignore 200 response
			if r.statusCode == http.StatusOK {
				continue
			}
			f.Rowln(fmt.Sprintf(" > %-3d %s", r.bID, txt.Shorten(r.URL, terminal.MinWidth)))
		}
	}

	took := fmt.Sprintf("%.2fs", d.Seconds())
	total := fmt.Sprintf("Total %s checked,", p.Blue(len(r)))
	f.Rowln().Footerln(total + " took " + p.Blue(took)).Flush()
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

	f := c.Frame()

	statusCategory := statusCode / 100
	switch statusCategory {
	case 2: // 2xx status codes
		f.Success(result.String() + "\n").Flush()
	case 3: // 3xx status codes
		f.Warning(result.String() + "\n").Flush()
	case 4: // 4xx status codes
		f.Error(result.String() + "\n").Flush()
	case 5: // 5xx status codes
		f.Error(result.String() + "\n").Flush()
	default: // Other status codes
		f.Midln(result.String()).Flush()
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
