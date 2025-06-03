package bookmark

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// HashURL generates a hash from a HashURL.
func HashURL(rawURL string) string {
	return generateHash(rawURL, 12)
}

// HashDomain generates a hash from the domain of a URL.
func HashDomain(rawURL string) (string, error) {
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
	return generateHash(data, 8)
}

// generateHash generates a hash from a string with the given length.
func generateHash(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}
