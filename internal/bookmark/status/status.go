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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

type Results struct {
	mu      sync.Mutex
	res     []*Response
	updated []*bookmark.Bookmark
}

func (r *Results) Add(res *Response) {
	r.mu.Lock()
	r.res = append(r.res, res)
	r.updated = append(r.updated, res.bookmark)
	r.mu.Unlock()
}

type Response struct {
	bookmark   *bookmark.Bookmark
	statusCode int
}

func (r *Response) String() string {
	p := ansi.NewPalette()
	colorStatus, colorCode := prettifyURLStatus(p, r.statusCode)

	statusCategory := r.statusCode / 100

	icons := ui.DefaultIconStyle

	var icon frame.IconStyle

	switch statusCategory {
	case 2: // 2xx status codes
		icon = icons.Success
	case 3: // 3xx status codes
		icon = icons.Warning
	case 4: // 4xx status codes
		icon = icons.Error
	case 5: // 5xx status codes
		icon = icons.Error
	default: // Other status codes
		icon = icons.Question
	}

	return fmt.Sprintf(
		"%s %s (%s %s) %s",
		icon,
		p.Bold.Sprintf("%-3d", r.bookmark.ID),
		colorCode,
		colorStatus,
		txt.Shorten(r.bookmark.URL, terminal.MinWidth),
	)
}

// Check checks the status of a slice of bookmarks.
func Check(ctx context.Context, c *ui.Console, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	start := time.Now()

	sp := rotato.New(
		rotato.WithPrefix("Checking URL Status"),
		rotato.WithMessage("processing..."),
		rotato.WithPrefixColor(rotato.StyleDim),
		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
		rotato.WithMessageColor(rotato.FgBrightBlue.With(rotato.StyleItalic)),
		rotato.WithFailSymbolColor(rotato.FgBrightRed.With(rotato.StyleBold)),
		rotato.WithFailMessageColor(rotato.FgBrightRed.With(rotato.StyleBold)),
	)
	sp.Start(ctx)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	var (
		current atomic.Uint32
		results = new(Results)
		total   = len(bs)
		p       = c.Palette()
	)

	sp.AddPrefixDecorator(func(mesg string) string {
		s := fmt.Sprintf("[%-*d/%d] ", 3, current.Load(), total)
		return p.BrightCyan.Wrap(s, p.Bold) + mesg
	})

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				old := *b
				res := makeRequest(ctx, b)

				sp.Print(res.String())
				current.Add(1)

				if res.statusCode != old.HTTPStatusCode {
					results.Add(&res)
				}

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sp.Done()

	duration := time.Since(start)
	printSummaryStatus(c, results.res, duration)

	return results.updated, nil
}

// prettifyURLStatus formats HTTP status codes into colored.
func prettifyURLStatus(p *ansi.Palette, code int) (status, statusCode string) {
	statusCategory := code / 100

	switch statusCategory {
	case 2: // 2xx status codes
		status = p.BrightGreen.Sprint("OK")
		statusCode = p.BrightGreen.Sprint(code)
	case 3: // 3xx status codes
		status = p.BrightYellow.Sprint("WA")
		statusCode = p.BrightYellow.Sprint(code)
	case 4: // 4xx status codes
		status = p.BrightRed.Sprint("ER")
		statusCode = p.BrightRed.Sprint(code)
	case 5: // 5xx status codes
		status = p.Red.Sprint("ER")
		statusCode = p.Red.Sprint(code)
	default: // Other status codes
		status = p.Yellow.Sprint("WA")
		statusCode = p.Yellow.Sprint(code)
	}

	return status, statusCode
}

// fmtSummary formats the summary of the status codes.
func fmtSummary(c *ui.Console, n, statusCode int, colorFn func(...any) string) string {
	total := fmt.Sprintf(colorFn("%-3d"), n)
	code := colorFn(statusCode)
	s := http.StatusText(statusCode)

	p := c.Palette()
	statusText := p.Italic.Sprint(s)
	if s == "" {
		statusText = p.Italic.Sprint("non-standard code")
	}

	return total + " URLs returned '" + statusText + "' (" + code + ")"
}

// printSummaryStatus prints a summary of HTTP status codes and their
// corresponding URLs.
func printSummaryStatus(c *ui.Console, r []*Response, d time.Duration) {
	if len(r) == 0 {
		return
	}

	p := c.Palette()
	codes := make(map[int][]Response)
	f := c.Frame()

	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	title := p.BrightYellow.
		Wrap("Summary URLs status\n", p.Bold)

	f.Rowln().CustomFunc(header, title)

	for _, res := range r {
		codes[res.statusCode] = append(codes[res.statusCode], *res)
	}

	for statusCode, res := range codes {
		n := len(res)

		statusCategory := statusCode / 100

		switch statusCategory {
		case 2: // 2xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.BrightGreen.With(p.Bold).Sprint))
		case 3: // 3xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.Yellow.With(p.Bold).Sprint))
		case 4: // 4xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.BrightRed.With(p.Bold).Sprint))
		case 5: // 5xx status codes
			f.Midln(fmtSummary(c, n, statusCode, p.Red.With(p.Bold).Sprint))
		default: // Other status codes
			f.Midln(fmtSummary(c, n, statusCode, p.Yellow.With(p.Bold).Sprint))
		}

		// adds URLs detail
		for _, r := range res {
			// ignore 200 response
			if r.statusCode == http.StatusOK {
				continue
			}

			f.Rowln(fmt.Sprintf(
				" > %-3d %s",
				r.bookmark.ID,
				txt.Shorten(r.bookmark.URL, terminal.MinWidth),
			))
		}
	}

	took := fmt.Sprintf("%.2fs\n", d.Seconds())
	total := fmt.Sprintf("Total %s checked,", p.Blue.Sprint(len(r)))
	header = func() string {
		return p.BrightBlue.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	f.Rowln().
		CustomFunc(header, total+" took "+p.Blue.Sprint(took)).
		Flush()
}

// buildResponse builds a Response from an HTTP response.
func buildResponse(b *bookmark.Bookmark, statusCode int) Response {
	b.HTTPStatusCode = statusCode
	b.HTTPStatusText = http.StatusText(statusCode)
	b.IsActive = statusCode >= 200 && statusCode <= 299
	b.LastStatusChecked = time.Now().Format("20060102150405")

	return Response{
		bookmark:   b,
		statusCode: statusCode,
	}
}

// handleRequestError handles errors from the HTTP request and determines the
// appropriate status code.
func handleRequestError(b *bookmark.Bookmark, err error) Response {
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

	return buildResponse(b, statusCode)
}

// makeRequest sends an HTTP GET request to the URL of the given bookmark and
// returns a response.
func makeRequest(ctx context.Context, b *bookmark.Bookmark) Response {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		b.URL,
		http.NoBody,
	)
	if err != nil {
		slog.Error("creating request", "url", b.URL, "error", err)
		return buildResponse(b, http.StatusNotFound)
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

	return buildResponse(b, resp.StatusCode)
}

func isNetworkUnreachableError(err error) bool {
	var netOpErr *net.OpError

	if errors.As(err, &netOpErr) {
		return netOpErr.Op == "connect" &&
			strings.Contains(
				netOpErr.Err.Error(),
				"network is unreachable",
			)
	}

	return false
}
