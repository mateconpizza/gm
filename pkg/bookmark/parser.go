package bookmark

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// hashDomain generates a hash from a domain.
func hashDomain(domain string) string {
	return generateHash(domain, 12)
}

// hashURL generates a hash from a hashURL.
func hashURL(rawURL string) string {
	return generateHash(rawURL, 12)
}

func generateHash(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}

// GenChecksum generates a checksum for the bookmark.
func GenChecksum(rawURL, title, desc, tags string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s", rawURL, title, desc, tags)
	return generateHash(data, 8)
}

// fromBytes unmarshals a bookmark from bytes.
func fromBytes(b []byte) (*Bookmark, error) {
	bj := BookmarkJSON{}
	if err := json.Unmarshal(b, &bj); err != nil {
		return nil, err
	}

	return NewFromJSON(&bj), nil
}

func toBytes(b *Bookmark) []byte {
	bj, err := json.MarshalIndent(b.JSON(), "", "  ")
	if err != nil {
		return nil
	}

	return bj
}

// ParseTags normalizes a string of tags by separating them by commas, sorting
// them and ensuring that the final string ends with a comma.
//
//	from: "tag1, tag2, tag3 tag"
//	to: "tag,tag1,tag2,tag3,"
func ParseTags(tags string) string {
	if tags == "" {
		return defaultTag
	}

	split := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	})
	sort.Strings(split)

	tags = strings.Join(UniqueTags(split), ",")
	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// UniqueTags returns a slice of unique tags.
func UniqueTags(t []string) []string {
	var (
		tags []string
		seen = make(map[string]bool)
	)

	for _, tag := range t {
		if tag == "" {
			continue
		}

		if !seen[tag] {
			seen[tag] = true

			tags = append(tags, tag)
		}
	}

	return tags
}

// domain extracts the domain from a URL.
func domain(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing url: %w", err)
	}

	// normalize domain
	domain := strings.ToLower(u.Hostname())

	return strings.TrimPrefix(domain, "www."), nil
}

// Validate validates the bookmark.
func Validate(b *Bookmark) error {
	if b.URL == "" {
		slog.Error("bookmark is invalid. URL is empty")
		return ErrBookmarkURLEmpty
	}

	if b.Tags == "," || b.Tags == "" || b.Tags == "notag" {
		slog.Error("bookmark is invalid. Tags are empty")
		return ErrBookmarkTagsEmpty
	}

	return nil
}

func makeReq(ctx context.Context, b *Bookmark) error {
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, http.NoBody)
	if err != nil {
		return err
	}

	client := http.DefaultClient
	//nolint:bodyclose //closing
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("creating request", slog.String("url", b.URL), slog.String("error", err.Error()))
		b.HTTPStatusCode = 0
		b.HTTPStatusText = "Network error"
		b.IsActive = false
		b.LastStatusChecked = time.Now().Format("20060102150405")
		return err
	}

	go func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("check status: closing body", "error", err)
		}
	}()

	b.HTTPStatusCode = resp.StatusCode
	b.HTTPStatusText = http.StatusText(resp.StatusCode)
	b.IsActive = resp.StatusCode >= 200 && resp.StatusCode <= 299
	b.LastStatusChecked = time.Now().Format("20060102150405")

	return nil
}
