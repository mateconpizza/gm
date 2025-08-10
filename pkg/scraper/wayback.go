package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

var (
	ErrAPIRequestFail     = errors.New("internet archive: API request failed")
	ErrNoVersionAvailable = errors.New("internet archive: no version available")
)

// api wayback machine availability api endpoint.
const api = "https://archive.org/wayback/available"

// waybackResponse represents the API response structure.
type waybackResponse struct {
	ArchivedSnapshots struct {
		Closest struct {
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
			Status    string `json:"status"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

type SnapshotInfo struct {
	URL       string `json:"url"`
	Timestamp string `json:"timestamp"`
}

// WaybackSnapshot fetches the latest snapshot metadata for the given URL from the
// Wayback Machine.
func WaybackSnapshot(s string) (*SnapshotInfo, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, api, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// query parameters
	q := req.URL.Query()
	q.Add("url", s)
	req.URL.RawQuery = q.Encode()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status: %d", ErrAPIRequestFail, resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var waybackResp waybackResponse
	if err := json.Unmarshal(body, &waybackResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if !waybackResp.ArchivedSnapshots.Closest.Available {
		return nil, fmt.Errorf("%w", ErrNoVersionAvailable)
	}

	return &SnapshotInfo{
		URL:       waybackResp.ArchivedSnapshots.Closest.URL,
		Timestamp: waybackResp.ArchivedSnapshots.Closest.Timestamp,
	}, nil
}
