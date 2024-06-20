package bookmark

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/slice"

	"golang.org/x/sync/semaphore"
)

var (
	C                     = format.Color
	ErrNetworkUnreachable = errors.New("network is unreachable")
)

type Response struct {
	URL        string
	id         int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	return prettyPrintURLStatus(r.statusCode, r.id, r.URL)
}

// CheckStatus checks the status of a slice of bookmarks
func CheckStatus(bs *slice.Slice[Bookmark]) error {
	// FIX: Split???
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

	var schedule = func(b Bookmark) error {
		wg.Add(1)
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}

		time.Sleep(50 * time.Millisecond)

		go func(b *Bookmark) {
			defer wg.Done()
			res := makeRequest(b, sem)
			responses.Add(&res)
		}(&b)

		return nil
	}

	if err := bs.ForEachErr(schedule); err != nil {
		return err
	}

	wg.Wait()

	duration := time.Since(start)
	prettyPrintStatus(*responses, duration)

	return nil
}

func prettifyNotFound(res *slice.Slice[Response]) (resLen int, msg string) {
	res.Filter(func(r Response) bool {
		return r.statusCode == http.StatusNotFound
	})

	n := res.Len()
	if n == 0 {
		return 0, ""
	}

	notFoundLenStr := C(strconv.Itoa(n)).Bold().Red()
	notFoundCode := C(strconv.Itoa(http.StatusNotFound)).Bold().Red()

	fmt.Printf("\n%s err detail:\n", notFoundLenStr)
	res.ForEach(func(r Response) {
		fmt.Println(r.String())
	})

	return n, fmt.Sprintf("  + %s urls return %s code\n", notFoundLenStr, notFoundCode)
}

func prettifyWithError(res *slice.Slice[Response]) (resLen int, msg string) {
	res.Filter(func(r Response) bool {
		return r.hasError && r.statusCode != http.StatusNotFound
	})
	n := res.Len()
	if n == 0 {
		return 0, ""
	}

	withErrorLenStr := C(strconv.Itoa(n)).Yellow().Bold()
	withNoErrorCode := C("200").Green().Bold()

	if n > 0 {
		fmt.Printf("\n%s warn detail:\n", withErrorLenStr)
		res.ForEach(func(r Response) {
			fmt.Println(r.String())
		})
	}

	return n, fmt.Sprintf("  + %s urls did not return a %s code\n", withErrorLenStr, withNoErrorCode)
}

func prettifyURLStatus(code int) (status, statusCode string) {
	switch code {
	case http.StatusNotFound:
		status = C("ER").Red().Bold().String()
		statusCode = C(strconv.Itoa(code)).Red().Bold().String()
	case http.StatusOK:
		status = C("OK").Green().Bold().String()
		statusCode = C(strconv.Itoa(code)).Green().Bold().String()
	default:
		status = C("WA").Yellow().Bold().String()
		statusCode = C(strconv.Itoa(code)).Yellow().Bold().String()
	}
	return status, statusCode
}

func prettyPrintURLStatus(statusCode, bID int, bURL string) string {
	var (
		c        = format.Color
		minWidth = 80
	)

	colorStatus, colorCode := prettifyURLStatus(statusCode)
	idStr := c(fmt.Sprintf("%-3d", bID)).Purple().Bold()
	id := c(":id:").Gray().String()
	id += fmt.Sprintf("[%s]", idStr)

	code := c(":code:").Gray().String()
	code += fmt.Sprintf("[%s]", colorCode)

	status := c(":status:").Gray().String()
	status += fmt.Sprintf("[%s]", colorStatus)

	url := c(":url:").Gray().String()
	url += format.ShortenString(bURL, minWidth)

	return fmt.Sprintf("%s%s%s%s", id, code, status, url)
}

// prettyPrintStatus prints a summary of the results of checking a slice of
// URLs.
func prettyPrintStatus(res slice.Slice[Response], duration time.Duration) {
	final := fmt.Sprintf("\n> %d urls were checked\n", res.Len())
	took := fmt.Sprintf("%.2fs\n", duration.Seconds())

	withErrLen, withErrMsg := prettifyWithError(&res)
	withNotFoundErr, withNoFoundErrMsg := prettifyNotFound(&res)

	withNoErrLen := res.Len() - withNotFoundErr - withErrLen
	if withNoErrLen > 0 {
		withNoErrorStr := strconv.Itoa(withNoErrLen)
		final += fmt.Sprintf(
			"  + %s urls return %s code\n",
			format.Color(withNoErrorStr).Blue().Bold().String(),
			format.Color("200").Green().Bold(),
		)
	}

	final += withErrMsg
	final += withNoFoundErrMsg

	final += fmt.Sprintf("  + it took %s\n", format.Color(took).Blue().Bold().String())
	fmt.Print(final)
}

// makeRequest sends an HTTP GET request to the URL of the given bookmark and
// returns a response.
//
// The function uses a weighted semaphore to limit the number of concurrent
// requests.
func makeRequest(b *Bookmark, sem *semaphore.Weighted) Response {
	// FIX: Split???
	defer sem.Release(1)

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		fmt.Printf("error creating request for %s: %v\n", b.URL, err)
		return Response{
			URL:        b.URL,
			id:         b.ID,
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

		s := C("error making request to").Yellow()
		fmt.Printf("%s %s: %v\n", s.String(), b.URL, err)
		return Response{
			URL:        b.URL,
			id:         b.ID,
			statusCode: http.StatusNotFound,
			hasError:   true,
		}
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("error closing response body:", err)
		}
	}()

	result := Response{
		URL:        b.URL,
		id:         b.ID,
		statusCode: resp.StatusCode,
		hasError:   resp.StatusCode != http.StatusOK,
	}

	fmt.Println(result.String())
	return result
}
