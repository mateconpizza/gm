package bookmark

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mateconpizza/gm/internal/format"
)

// HashURL generates a hash from a HashURL.
func HashURL(rawURL string) string {
	return format.GenerateHash(rawURL, 12)
}

// HashDomain generates a hash from a domain.
func HashDomain(rawURL string) (string, error) {
	domain, err := Domain(rawURL)
	if err != nil {
		return "", err
	}
	return format.GenerateHash(domain, 12), nil
}

// Domain extracts the domain from a URL.
func Domain(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing url: %w", err)
	}
	// normalize domain
	domain := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(domain, "www."), nil
}

// Checksum generates a checksum for the bookmark.
func Checksum(rawURL, title, desc, tags string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s", rawURL, title, desc, tags)
	return format.GenerateHash(data, 8)
}
