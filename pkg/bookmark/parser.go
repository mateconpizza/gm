package bookmark

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"sort"
	"strings"
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

// genChecksum generates a checksum for the bookmark.
func genChecksum(rawURL, title, desc, tags, notes string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s|notes:%s", rawURL, title, desc, tags, notes)
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
		return DefaultTag
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

// Domain extracts the Domain from a URL.
func Domain(rawURL string) (string, error) {
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

	if b.Tags == "," || b.Tags == "" {
		slog.Error("bookmark is invalid. Tags are empty")
		return ErrBookmarkTagsEmpty
	}

	return nil
}

func ValidateChecksumJSON(b *BookmarkJSON) bool {
	tags := ParseTags(strings.Join(b.Tags, ","))
	return b.Checksum == genChecksum(b.URL, b.Title, b.Desc, tags, b.Notes)
}

func Fields() []string {
	t := reflect.TypeFor[Bookmark]()
	fields := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		if tag := t.Field(i).Tag.Get("db"); tag != "" && tag != "-" {
			fields = append(fields, tag)
		}
	}
	return fields
}

// CopyMetadata copies non-content bookmark metadata from src to dst.
func CopyMetadata(dst, src *Bookmark) *Bookmark {
	// Identity
	dst.ID = src.ID

	// Data
	dst.Notes = src.Notes

	// Timestamps
	dst.CreatedAt = src.CreatedAt
	dst.LastVisit = src.LastVisit
	dst.LastStatusChecked = src.LastStatusChecked

	// Usage stats
	dst.VisitCount = src.VisitCount
	dst.Favorite = src.Favorite

	// Link health
	dst.HTTPStatusCode = src.HTTPStatusCode
	dst.HTTPStatusText = src.HTTPStatusText
	dst.IsActive = src.IsActive

	// Media / enrichment
	dst.FaviconURL = src.FaviconURL

	// Archive metadata
	dst.ArchiveURL = src.ArchiveURL
	dst.ArchiveTimestamp = src.ArchiveTimestamp

	return dst
}
