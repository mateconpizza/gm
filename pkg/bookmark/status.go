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

	"gomarks/pkg/format"
	"gomarks/pkg/terminal"

	"golang.org/x/sync/semaphore"
)

var ErrNetworkUnreachable = errors.New("network is unreachable")

type Response struct {
	URL        string
	id         int
	statusCode int
	hasError   bool
}

func (r *Response) String() string {
	return prettyPrintURLStatus(r.statusCode, r.id, r.URL)
}

func filterResponse(res *[]Response, filterFn func(Response) bool) (ret []Response) {
	for _, s := range *res {
		if filterFn(s) {
			ret = append(ret, s)
		}
	}
	return ret
}

func prettifyNotFound(res *[]Response) (resLen int, msg string) {
	filterNotFoundFn := func(r Response) bool { return r.statusCode == http.StatusNotFound }
	withNotFoundErr := filterResponse(res, filterNotFoundFn)

	if len(withNotFoundErr) == 0 {
		return 0, ""
	}

	notFoundLenStr := format.Text(strconv.Itoa(len(withNotFoundErr))).Bold().Red()
	notFoundCode := format.Text("404").Bold().Red()

	fmt.Printf("\n%s err detail:\n", notFoundLenStr)
	for _, r := range withNotFoundErr {
		fmt.Println(r.String())
	}

	return len(withNotFoundErr), fmt.Sprintf("  + %s urls return %s code\n", notFoundLenStr, notFoundCode)
}

func prettifyWithError(res *[]Response) (resLen int, msg string) {
	filterErrResFn := func(r Response) bool { return r.hasError && r.statusCode != http.StatusNotFound }
	withErr := filterResponse(res, filterErrResFn)

	if len(withErr) == 0 {
		return 0, ""
	}

	withErrorLenStr := format.Text(strconv.Itoa(len(withErr))).Yellow().Bold()
	withNoErrorCode := format.Text("200").Green().Bold()

	if len(withErr) > 0 {
		fmt.Printf("\n%s warn detail:\n", withErrorLenStr)
		for _, r := range withErr {
			fmt.Println(r.String())
		}
	}

	return len(withErr), fmt.Sprintf("  + %s urls did not return a %s code\n", withErrorLenStr, withNoErrorCode)
}

func prettifyURLStatus(code int) (status, statusCode string) {
	switch code {
	case http.StatusNotFound:
		status = format.Text("ER").Red().Bold().String()
		statusCode = format.Text(strconv.Itoa(code)).Red().Bold().String()
	case http.StatusOK:
		status = format.Text("OK").Green().Bold().String()
		statusCode = format.Text(strconv.Itoa(code)).Green().Bold().String()
	default:
		status = format.Text("WA").Yellow().Bold().String()
		statusCode = format.Text(strconv.Itoa(code)).Yellow().Bold().String()
	}
	return status, statusCode
}

func prettyPrintURLStatus(statusCode, bID int, url string) string {
	colorStatus, colorCode := prettifyURLStatus(statusCode)

	idStr := format.Text(fmt.Sprintf("%-3d", bID)).Purple().Bold()
	id := format.Text(":ID:").Gray().String()
	id += fmt.Sprintf("[%s]", idStr)

	code := format.Text(":Code:").Gray().String()
	code += fmt.Sprintf("[%s]", colorCode)

	status := format.Text(":Status:").Gray().String()
	status += fmt.Sprintf("[%s]", colorStatus)

	u := format.Text(":URL:").Gray().String()
	u += format.ShortenString(url, terminal.Settings.MinWidth)

	return fmt.Sprintf("%s%s%s%s", id, code, status, u)
}

func prettyPrintStatus(res []Response, duration time.Duration) {
	final := fmt.Sprintf("\n> %d urls were checked\n", len(res))

	took := fmt.Sprintf("%.2fs\n", duration.Seconds())

	withErrLen, withErrMsg := prettifyWithError(&res)
	withNotFoundErr, withNoFoundErrMsg := prettifyNotFound(&res)

	withNoErrLen := len(res) - withNotFoundErr - withErrLen
	if withNoErrLen > 0 {
		withNoErrorStr := strconv.Itoa(withNoErrLen)
		final += fmt.Sprintf(
			"  + %s urls return %s code\n",
			format.Text(withNoErrorStr).Blue().Bold().String(),
			format.Text("200").Green().Bold(),
		)
	}

	final += withErrMsg
	final += withNoFoundErrMsg

	final += fmt.Sprintf("  + it took %s\n", format.Text(took).Blue().Bold().String())
	fmt.Print(final)
}

func CheckStatus(bs *[]Bookmark) error {
	const maxConRequests = 50
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
			id:         b.ID,
			statusCode: 404,
			hasError:   true,
		}
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connect: network is unreachable") {
			log.Println(err)
		}

		fmt.Printf("Error making request to %s: %v\n", b.URL, err)
		return Response{
			URL:        b.URL,
			id:         b.ID,
			statusCode: 404,
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
		id:         b.ID,
		statusCode: resp.StatusCode,
		hasError:   resp.StatusCode != http.StatusOK,
	}

	fmt.Println(result.String())
	return result
}
