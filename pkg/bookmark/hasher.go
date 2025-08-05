package bookmark

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
