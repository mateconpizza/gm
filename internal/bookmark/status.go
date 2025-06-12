package bookmark

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

	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

type Response struct {
	URL        string
	bID        int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	id := color.Gray("ID:").String()
	id += fmt.Sprintf("[%s]", color.Purple(fmt.Sprintf("%-3d", r.bID)).Bold())

	colorStatus, colorCode := prettifyURLStatus(r.statusCode)
	code := color.Gray(":Code:").String()
	code += fmt.Sprintf("[%s]", colorCode)

	status := color.Gray(":Status:").String()
	status += fmt.Sprintf("[%s]", colorStatus)

	url := color.Gray(":URL:").String()
	url += txt.Shorten(r.URL, terminal.MinWidth)

	return fmt.Sprintf("%s%s%s%s", id, code, status, url)
}

// Status checks the status of a slice of bookmarks.
func Status(bs *slice.Slice[Bookmark]) error {
	const maxConRequests = 25
	var (
		responses = slice.New[Response]()
		sem       *semaphore.Weighted
		start     time.Time
		wg        sync.WaitGroup
	)

	sem = semaphore.NewWeighted(int64(maxConRequests))
	start = time.Now()
	ctx := context.Background()

	schedule := func(b Bookmark) error {
		wg.Add(1)
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}

		time.Sleep(50 * time.Millisecond)

		go func(b *Bookmark) {
			defer wg.Done()
			res := makeRequest(b, ctx, sem)
			responses.Append(res)
		}(&b)

		return nil
	}

	if err := bs.ForEachErr(schedule); err != nil {
		return fmt.Errorf("%w", err)
	}

	wg.Wait()

	duration := time.Since(start)
	printSummaryStatus(responses, duration)

	return nil
}

// prettifyURLStatus formats HTTP status codes into colored.
func prettifyURLStatus(code int) (status, statusCode string) {
	switch code {
	case http.StatusNotFound:
		status = color.Red("ER").Bold().String()
		statusCode = color.Red("404").Bold().String()
	case http.StatusOK:
		status = color.BrightGreen("OK").Bold().String()
		statusCode = color.BrightGreen("200").Bold().String()
	default:
		status = color.Yellow("WA").Bold().String()
		statusCode = color.Yellow(code).Bold().String()
	}

	return status, statusCode
}

// fmtSummary formats the summary of the status codes.
func fmtSummary(n, statusCode int, c color.ColorFn) string {
	total := fmt.Sprintf(c("%-3d").Bold().String(), n)
	code := c(statusCode).Bold().String()
	s := http.StatusText(statusCode)
	statusText := color.Text(s).Italic().String()
	if s == "" {
		statusText = color.Text("non-standard code").Italic().String()
	}

	return total + " URLs returned '" + statusText + "' (" + code + ")"
}

// printSummaryStatus prints a summary of HTTP status codes and their
// corresponding URLs.
func printSummaryStatus(r *slice.Slice[Response], d time.Duration) {
	var (
		f     = frame.New(frame.WithColorBorder(color.Gray)).Ln()
		codes = make(map[int][]Response)
	)

	f.Header(color.BrightGreen("Summary URLs status:\n").Bold().String())

	r.ForEach(func(r Response) {
		codes[r.statusCode] = append(codes[r.statusCode], r)
	})

	for statusCode, res := range codes {
		n := len(res)

		switch statusCode {
		case http.StatusNotFound,
			http.StatusGone,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable:
			f.Mid(fmtSummary(n, statusCode, color.Red)).Ln()
		case http.StatusForbidden, http.StatusTooManyRequests:
			f.Mid(fmtSummary(n, statusCode, color.Orange)).Ln()
		case http.StatusOK:
			f.Mid(fmtSummary(n, statusCode, color.BrightGreen)).Ln()
		default:
			f.Mid(fmtSummary(n, statusCode, color.Yellow)).Ln()
		}

		// adds URLs detail
		for _, r := range res {
			if r.statusCode == http.StatusOK {
				continue
			}
			bid := fmt.Sprintf(color.BrightGray("%-3d").String(), r.bID)
			url := color.Gray(txt.Shorten(r.URL, terminal.MinWidth)).Italic().String()
			f.Row(fmt.Sprintf(" %s %s", bid, url)).Ln()
		}
	}

	took := fmt.Sprintf("%.2fs", d.Seconds())
	total := fmt.Sprintf("Total %s checked,", color.Blue(r.Len()).Bold())
	f.Row("\n").Footer(total + " took " + color.Blue(took).Bold().String() + "\n")
	f.Flush()
}

// buildResponse builds a Response from an HTTP response.
func buildResponse(b *Bookmark, statusCode int, hasError bool) Response {
	result := Response{
		URL:        b.URL,
		bID:        b.ID,
		statusCode: statusCode,
		hasError:   hasError,
	}
	fmt.Println(result.String())

	return result
}

// handleRequestError handles errors from the HTTP request and determines the
// appropriate status code.
func handleRequestError(b *Bookmark, err error) Response {
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

	return buildResponse(b, statusCode, true)
}

// makeRequest sends an HTTP GET request to the URL of the given bookmark and
// returns a response.
//
// The function uses a weighted semaphore to limit the number of concurrent
// requests.
func makeRequest(b *Bookmark, ctx context.Context, sem *semaphore.Weighted) Response {
	// FIX: Split this function
	defer sem.Release(1)

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		slog.Error("creating request", slog.String("url", b.URL), slog.String("error", err.Error()))
		return buildResponse(b, http.StatusNotFound, true)
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return handleRequestError(b, err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("error closing response body:", err)
		}
	}()

	return buildResponse(b, resp.StatusCode, resp.StatusCode != http.StatusOK)
}

func isNetworkUnreachableError(err error) bool {
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		return netOpErr.Op == "connect" &&
			strings.Contains(netOpErr.Err.Error(), "network is unreachable")
	}

	return false
}
