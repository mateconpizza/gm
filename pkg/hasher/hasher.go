// Package hasher provides a utility for generating hash values.
package hasher

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func Domain(domain string) (string, error) {
	return Generate(domain, 12), nil
}

// URL generates a hash from a URL.
func URL(rawURL string) string {
	return Generate(rawURL, 12)
}

func Generate(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}

// GenChecksum generates a checksum for the bookmark.
func GenChecksum(rawURL, title, desc, tags string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s", rawURL, title, desc, tags)
	return Generate(data, 8)
}
