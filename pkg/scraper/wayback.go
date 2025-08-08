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

// WaybackResponse represents the API response structure.
type WaybackResponse struct {
	ArchivedSnapshots struct {
		Closest struct {
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
			Status    string `json:"status"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

// InternetArchive fetches the Wayback Machine URL for a given URL.
func InternetArchive(originalURL string) (string, error) {
	// Wayback Machine Availability API endpoint
	apiURL := "https://archive.org/wayback/available"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// query parameters
	q := req.URL.Query()
	q.Add("url", originalURL)
	req.URL.RawQuery = q.Encode()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status: %d", ErrAPIRequestFail, resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var waybackResp WaybackResponse
	if err := json.Unmarshal(body, &waybackResp); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check if archived version is available
	if !waybackResp.ArchivedSnapshots.Closest.Available {
		return "", fmt.Errorf("%w", ErrNoVersionAvailable)
	}

	return waybackResp.ArchivedSnapshots.Closest.URL, nil
}
