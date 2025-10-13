// Package wayback provides a client for the Internet Archive Wayback Machine API.
package wayback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	ErrAPIRequestFail     = errors.New("wayback machine: API request failed")
	ErrAlreadyArchived    = errors.New("wayback machine: URL already has archive")
	ErrNoSnapshots        = errors.New("wayback machine: no snapshots found")
	ErrNoVersionAvailable = errors.New("wayback machine: no version available")
	ErrSaveFailed         = errors.New("wayback machine: failed to save snapshot")
	ErrTooManyRecords     = errors.New("wayback machine: too many records")
)

const (
	// availableAPI is the Wayback Machine Availability API endpoint.
	// Used to check if a specific URL is archived and get the closest snapshot.
	//
	// Example: https://archive.org/wayback/available?url=example.com
	availableAPI = "https://archive.org/wayback/available"

	// cdxAPI is the Wayback Machine CDX Server API endpoint.
	// Provides low-level access to the capture index for advanced searching,
	// filtering, and metadata retrieval about archived pages.
	//
	// Example: https://web.archive.org/cdx/search/cdx?url=example.com&output=json
	cdxAPI = "https://web.archive.org/cdx/search/cdx"
)

const (
	// defaultTimeout timeout specifies a time limit for requests made by the
	// http.Client.
	defaultTimeout = 30 * time.Second

	// MaxItems sets the max items to fetch.
	MaxItems = 10
)

type OptFn func(*Options)

type Options struct {
	limit int
	year  int
}

// WaybackMachine provides methods to query the Internet Archive Wayback Machine.
type WaybackMachine struct {
	*Options
	client *http.Client
}

// New creates a new WaybackMachine with sensible defaults.
func New(opts ...OptFn) *WaybackMachine {
	o := &Options{}
	for _, fn := range opts {
		fn(o)
	}

	if o.limit == 0 {
		o.limit = MaxItems
	}

	return &WaybackMachine{
		Options: o,
		client:  &http.Client{Timeout: defaultTimeout},
	}
}

// WithLimit sets the maximum number of snapshots to fetch.
func WithLimit(n int) OptFn {
	return func(o *Options) {
		o.limit = n
	}
}

// WithByYear sets the year of the snapshots to fetch.
func WithByYear(n int) OptFn {
	return func(o *Options) {
		o.year = n
	}
}

// AvailableSnapshot represents the Wayback "available" API response.
type AvailableSnapshot struct {
	ArchivedSnapshots struct {
		Closest struct {
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
			Status    string `json:"status"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

// SnapshotInfo represents a single CDX snapshot record.
type SnapshotInfo struct {
	ArchiveURL       string `json:"archive_url"`
	ArchiveTimestamp string `json:"timestamp"`
}

// ClosestSnapshot returns the closest archived version of a URL.
func (wm *WaybackMachine) ClosestSnapshot(ctx context.Context, urlStr string) (*SnapshotInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, availableAPI, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("url", urlStr)
	req.URL.RawQuery = q.Encode()

	resp, err := wm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrAPIRequestFail, resp.StatusCode)
	}

	var data AvailableSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if !data.ArchivedSnapshots.Closest.Available {
		return nil, ErrNoVersionAvailable
	}

	return &SnapshotInfo{
		ArchiveURL:       data.ArchivedSnapshots.Closest.URL,
		ArchiveTimestamp: data.ArchivedSnapshots.Closest.Timestamp,
	}, nil
}

// Snapshots fetches the last N snapshots for a URL.
func (wm *WaybackMachine) Snapshots(ctx context.Context, urlStr string) ([]SnapshotInfo, error) {
	endpoint := fmt.Sprintf("%s?%s", cdxAPI, wm.params(urlStr))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := wm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrAPIRequestFail, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var raw [][]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if len(raw) <= 1 {
		return nil, ErrNoSnapshots
	}

	snapshots := make([]SnapshotInfo, 0, len(raw))
	for _, row := range raw[1:] {
		if len(row) < 3 {
			continue
		}

		snapshots = append(snapshots, SnapshotInfo{
			ArchiveURL:       fmt.Sprintf("https://web.archive.org/web/%s/%s", row[1], row[2]),
			ArchiveTimestamp: row[1],
		})
	}

	return snapshots, nil
}

// SaveSnapshot requests the Wayback Machine to archive a URL (create new snapshot).
func (wm *WaybackMachine) SaveSnapshot(ctx context.Context, urlStr string) (string, error) {
	saveURL := "https://web.archive.org/save/" + urlStr

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, saveURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := wm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to save snapshot: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	// The response redirects to the archived page
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return resp.Request.URL.String(), nil
	}

	return "", fmt.Errorf("%w status: %d", ErrSaveFailed, resp.StatusCode)
}

func (wm *WaybackMachine) params(u string) string {
	p := url.Values{}
	p.Set("url", u)
	p.Set("output", "json")
	p.Set("limit", strconv.Itoa(wm.limit))
	p.Set("sort", "reverse")
	p.Set("filter", "statuscode:200")

	if wm.year > 0 {
		p.Set("from", strconv.Itoa(wm.year))
		p.Set("to", strconv.Itoa(wm.year))
	}

	return p.Encode()
}
