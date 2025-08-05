// Package bookmark contains the bookmark record.
package bookmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultTag = "notag"

var (
	ErrDuplicate       = errors.New("bookmark already exists")
	ErrInvalid         = errors.New("bookmark invalid")
	ErrInvalidID       = errors.New("invalid bookmark id")
	ErrInvalidInput    = errors.New("invalid input")
	ErrNotFound        = errors.New("no bookmark found")
	ErrNotSelected     = errors.New("no bookmark selected")
	ErrTagsEmpty       = errors.New("tags cannot be empty")
	ErrURLEmpty        = errors.New("URL cannot be empty")
	ErrUnknownField    = errors.New("bookmark field unknown")
	ErrInvalidChecksum = errors.New("invalid checksum")
)

// Bookmark represents a bookmark.
type Bookmark struct {
	ID           int    `db:"id"            json:"id"`
	URL          string `db:"url"           json:"url"`
	Tags         string `db:"tags"          json:"tags"`
	Title        string `db:"title"         json:"title"`
	Desc         string `db:"desc"          json:"desc"`
	CreatedAt    string `db:"created_at"    json:"created_at"`
	LastVisit    string `db:"last_visit"    json:"last_visit"`
	UpdatedAt    string `db:"updated_at"    json:"updated_at"`
	VisitCount   int    `db:"visit_count"   json:"visit_count"`
	Favorite     bool   `db:"favorite"      json:"favorite"`
	FaviconURL   string `db:"favicon_url"   json:"favicon_url"`
	FaviconLocal string `db:"favicon_local" json:"favicon_local"`
	ArchiveURL   string `db:"archive_url"   json:"archive_url"`
	Checksum     string `db:"checksum"      json:"checksum"`
}

type BookmarkJSON struct {
	ID           int      `json:"id"`
	URL          string   `json:"url"`
	Tags         []string `json:"tags"`
	Title        string   `json:"title"`
	Desc         string   `json:"desc"`
	CreatedAt    string   `json:"created_at"`
	LastVisit    string   `json:"last_visit"`
	UpdatedAt    string   `json:"updated_at"`
	VisitCount   int      `json:"visit_count"`
	Favorite     bool     `json:"favorite"`
	FaviconURL   string   `json:"favicon_url"`
	FaviconLocal string   `json:"favicon_local"`
	ArchiveURL   string   `json:"archive_url"`
	Checksum     string   `json:"checksum"`
}

func (b *Bookmark) JSON() *BookmarkJSON {
	t := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool {
			return r == ',' || r == ' '
		})
	}

	return &BookmarkJSON{
		ID:         b.ID,
		URL:        b.URL,
		Title:      b.Title,
		Desc:       b.Desc,
		Tags:       t(b.Tags),
		CreatedAt:  b.CreatedAt,
		LastVisit:  b.LastVisit,
		UpdatedAt:  b.UpdatedAt,
		VisitCount: b.VisitCount,
		Favorite:   b.Favorite,
		FaviconURL: b.FaviconURL,
		ArchiveURL: b.ArchiveURL,
		Checksum:   b.Checksum,
	}
}

func (b *Bookmark) Bytes() []byte {
	return toBytes(b)
}

// Field returns the value of a field.
func (b *Bookmark) Field(f string) (string, error) {
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
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownField, f)
	}

	return s, nil
}

// Equals reports whether b and o have the same URL, Tags, Title and Desc.
func (b *Bookmark) Equals(o *Bookmark) bool {
	if b == nil || o == nil {
		return b == o
	}

	return b.URL == o.URL &&
		b.Tags == o.Tags &&
		b.Title == o.Title &&
		b.Desc == o.Desc
}

func (b *Bookmark) Buffer() []byte {
	return fmt.Appendf(nil, `# URL: (required)
%s
# Title: (leave an empty line for web fetch)
%s
# Tags: (comma separated)
%s
# Description:
%s

# end ------------------------------------------------------------------`,
		b.URL, b.Title, ParseTags(b.Tags), b.Desc)
}

// GenChecksum generates a checksum for the bookmark.
//
// It uses the URL, Title, Description and Tags.
func (b *Bookmark) GenChecksum() {
	b.Checksum = GenChecksum(b.URL, b.Title, b.Desc, b.Tags)
}

// HashPath returns the hash path of a bookmark.
//
//	hashDomain + Checksum
func (b *Bookmark) HashPath() (string, error) {
	s, err := domain(b.URL)
	if err != nil {
		return "", err
	}

	return filepath.Join(hashDomain(s), b.Checksum), nil
}

// HashURL returns the hash of a bookmark URL.
func (b *Bookmark) HashURL() string {
	return hashURL(b.URL)
}

// Domain returns the domain of a bookmark.
func (b *Bookmark) Domain() (string, error) {
	return domain(b.URL)
}

// JSONPath returns the path to the JSON file.
//
//	domain -> urlHash.json
func (b *Bookmark) JSONPath() (string, error) {
	domain, err := domain(b.URL)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	urlHash := hashURL(b.URL)

	return filepath.Join(domain, urlHash+".json"), nil
}

// GPGPath returns the path to the GPG file.
//
//	domainHash -> urlHash.gpg
func (b *Bookmark) GPGPath(ext string) (string, error) {
	domain, err := domain(b.URL)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return filepath.Join(domain, b.Checksum+ext), nil
}

// New creates a new bookmark.
func New() *Bookmark {
	return &Bookmark{}
}

func NewJSON() *BookmarkJSON {
	return &BookmarkJSON{}
}

func (bj *BookmarkJSON) Buffer() []byte {
	tags := strings.Join(bj.Tags, ",")
	return fmt.Appendf(nil, `# URL: (required)
%s
# Title: (leave an empty line for web fetch)
%s
# Tags: (comma separated)
%s
# Description:
%s

# end ------------------------------------------------------------------`,
		bj.URL, bj.Title, ParseTags(tags), bj.Desc)
}

func NewFromJSON(j *BookmarkJSON) *Bookmark {
	b := New()
	b.ID = j.ID
	b.URL = j.URL
	b.Title = j.Title
	b.Desc = j.Desc
	b.Tags = ParseTags(strings.Join(j.Tags, ","))
	b.CreatedAt = j.CreatedAt
	b.LastVisit = j.LastVisit
	b.UpdatedAt = j.UpdatedAt
	b.VisitCount = j.VisitCount
	b.Favorite = j.Favorite
	b.FaviconURL = j.FaviconURL
	b.Checksum = j.Checksum

	return b
}

func NewFromBuffer(buf []byte) (*Bookmark, error) {
	return fromBytes(buf)
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

	tags = strings.Join(uniqueTags(split), ",")
	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// uniqueTags returns a slice of unique tags.
func uniqueTags(t []string) []string {
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
		return ErrURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		slog.Error("bookmark is invalid. Tags are empty")
		return ErrTagsEmpty
	}

	return nil
}
