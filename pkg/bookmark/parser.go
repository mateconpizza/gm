package bookmark

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Field returns the value of a field.
func Field(f string, b *Bookmark) (string, error) {
	var s string

	switch f {
	case "id", "i", "1":
		s = strconv.Itoa(b.ID)
	case "url", "u", "2":
		s = b.URL
	case "title", "t", "3":
		s = b.Title
	case "tags", "T", "4":
		s = b.Tags
	case "desc", "d", "5":
		s = b.Desc
	case "notes", "n", "6":
		s = b.Notes
	case "domain", "dom", "7":
		d, err := b.Domain()
		if err != nil {
			return "", err
		}
		return d, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrBookmarkUnknownField, f)
	}

	return s, nil
}

// NewFromBuffer unmarshals a bookmark from bytes.
func NewFromBuffer(buf []byte) (*Bookmark, error) {
	bj := NewRecord()
	if err := json.Unmarshal(buf, &bj); err != nil {
		return nil, err
	}

	return bj.Bookmark(), nil
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
		return ErrBookmarkURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		return ErrBookmarkTagsEmpty
	}

	return nil
}

func Fields() []string {
	t := reflect.TypeFor[Bookmark]()
	fields := make([]string, 0, t.NumField())
	for field := range t.Fields() {
		if tag := field.Tag.Get("db"); tag != "" && tag != "-" {
			fields = append(fields, tag)
		}
	}
	return fields
}

// hashDomain generates a hash from a domain.
func hashDomain(domain string) string { return generateHash(domain, 12) }

// hashURL generates a hash from a hashURL.
func hashURL(rawURL string) string { return generateHash(rawURL, 12) }

func generateHash(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}

// genChecksum generates a checksum for the bookmark.
func genChecksum(rawURL, title, desc, tags, notes string) string {
	data := fmt.Sprintf("u:%s|t:%s|d:%s|tags:%s|notes:%s", rawURL, title, desc, tags, notes)
	return generateHash(data, 8)
}
