package bookmark

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util/frame"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

type Response struct {
	URL        string
	bID        int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	id := color.Gray("id:").String()
	id += fmt.Sprintf("[%s]", color.Purple(fmt.Sprintf("%-3d", r.bID)).Bold())

	colorStatus, colorCode := prettifyURLStatus(r.statusCode)
	code := color.Gray(":code:").String()
	code += fmt.Sprintf("[%s]", colorCode)

	status := color.Gray(":status:").String()
	status += fmt.Sprintf("[%s]", colorStatus)

	url := color.Gray(":url:").String()
	url += format.ShortenString(r.URL, terminal.MinWidth)

	return fmt.Sprintf("%s%s%s%s", id, code, status, url)
}

// CheckStatus checks the status of a slice of bookmarks.
func CheckStatus(bs *slice.Slice[Bookmark]) error {
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
			responses.Append(&res)
		}(&b)

		return nil
	}

	if err := bs.ForEachErr(schedule); err != nil {
		return fmt.Errorf("%w", err)
	}

	wg.Wait()

	duration := time.Since(start)
	printSummaryStatus(*responses, duration)

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
	statusText := color.Text(http.StatusText(statusCode)).Italic().String()

	return total + " urls returned '" + statusText + "' (" + code + ")"
}

// printSummaryStatus prints a summary of HTTP status codes and their
// corresponding URLs.
func printSummaryStatus(r slice.Slice[Response], d time.Duration) {
	var (
		f     = frame.New(frame.WithColorBorder(color.Gray)).Newline()
		codes = make(map[int][]Response)
	)

	f.Header(color.BrightGreen("Summary URLs status:").Bold().String())

	r.ForEach(func(r Response) {
		codes[r.statusCode] = append(codes[r.statusCode], r)
	})

	for statusCode, res := range codes {
		n := len(res)

		switch statusCode {
		case http.StatusNotFound:
			f.Mid(fmtSummary(n, statusCode, color.Red))
		case http.StatusForbidden, http.StatusTooManyRequests:
			f.Mid(fmtSummary(n, statusCode, color.Orange))
		case http.StatusOK:
			f.Mid(fmtSummary(n, statusCode, color.BrightGreen))
		default:
			f.Mid(fmtSummary(n, statusCode, color.Yellow))
		}

		// adds URLs detail
		for _, r := range res {
			if r.statusCode == http.StatusOK {
				continue
			}
			bid := fmt.Sprintf(color.Gray("%-3d").Bold().String(), r.bID)
			url := color.Gray(format.ShortenString(r.URL, terminal.MinWidth)).Italic().String()
			f.Row(fmt.Sprintf(" %s %s", bid, url))
		}
	}

	took := fmt.Sprintf("%.2fs", d.Seconds())
	total := fmt.Sprintf("Total %s checked,", color.Blue(r.Len()).Bold())
	f.Row().Footer(total + " took " + color.Blue(took).Bold().String())

	f.Newline().Render()
}

// makeRequest sends an HTTP GET request to the URL of the given bookmark and
// returns a response.
//
// The function uses a weighted semaphore to limit the number of concurrent
// requests.
func makeRequest(b *Bookmark, ctx context.Context, sem *semaphore.Weighted) Response {
	// FIX: Split this function
	defer sem.Release(1)

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		log.Printf("error creating request for %s: %v\n", b.URL, err)
		return Response{
			URL:        b.URL,
			bID:        b.ID,
			statusCode: http.StatusNotFound,
			hasError:   true,
		}
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connect: network is unreachable") {
			log.Println(err)
		}

		result := Response{
			URL:        b.URL,
			bID:        b.ID,
			statusCode: http.StatusNotFound,
			hasError:   true,
		}

		fmt.Println(result.String())

		return result
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("error closing response body:", err)
		}
	}()

	result := Response{
		URL:        b.URL,
		bID:        b.ID,
		statusCode: resp.StatusCode,
		hasError:   resp.StatusCode != http.StatusOK,
	}

	fmt.Println(result.String())

	return result
}
