// Package bookmark contains the bookmark record.
package bookmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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
	ID                int    `db:"id"                json:"id"`
	URL               string `db:"url"               json:"url"`               // URL of the bookmark.
	Tags              string `db:"tags"              json:"tags"`              // Tags for the bookmark, stored as a comma-separated string.
	Title             string `db:"title"             json:"title"`             // Title of the bookmark, retrieved from the website's metadata.
	Desc              string `db:"desc"              json:"desc"`              // Description of the bookmark.
	CreatedAt         string `db:"created_at"        json:"created_at"`        // Timestamp when the bookmark was created.
	LastVisit         string `db:"last_visit"        json:"last_visit"`        // Timestamp of the last time the bookmark was visited.
	UpdatedAt         string `db:"updated_at"        json:"updated_at"`        // Timestamp of the last time the bookmark record was updated.
	VisitCount        int    `db:"visit_count"       json:"visit_count"`       // The number of times the bookmark has been visited.
	Favorite          bool   `db:"favorite"          json:"favorite"`          // Boolean indicating if the bookmark is marked as a favorite.
	FaviconURL        string `db:"favicon_url"       json:"favicon_url"`       // URL for the bookmark's favicon.
	FaviconLocal      string `db:"favicon_local"     json:"favicon_local"`     // Local path to the cached favicon file.
	Checksum          string `db:"checksum"          json:"checksum"`          // Checksum or hash (URL, Title, Description and Tags)
	ArchiveURL        string `db:"archive_url"       json:"archive_url"`       // Internet Archive URL
	ArchiveTimestamp  string `db:"archive_timestamp" json:"archive_timestamp"` // Internet Archive timestamp
	LastStatusChecked string `db:"last_checked"      json:"last_checked"`      // Last checked timestamp.
	HTTPStatusCode    int    `db:"status_code"       json:"status_code"`       // HTTP status code (200, 404, etc.)
	HTTPStatusText    string `db:"status_text"       json:"status_text"`       // OK, Not Found, etc
	IsActive          bool   `db:"is_active"         json:"is_active"`         // true if the URL is active (200-299)
}

type BookmarkJSON struct {
	// FIX: remove this struct
	ID                int      `json:"id"`
	URL               string   `json:"url"`
	Tags              []string `json:"tags"`
	Title             string   `json:"title"`
	Desc              string   `json:"desc"`
	CreatedAt         string   `json:"created_at"`
	LastVisit         string   `json:"last_visit"`
	UpdatedAt         string   `json:"updated_at"`
	VisitCount        int      `json:"visit_count"`
	Favorite          bool     `json:"favorite"`
	FaviconURL        string   `json:"favicon_url"`
	FaviconLocal      string   `json:"favicon_local"`
	Checksum          string   `json:"checksum"`
	ArchiveURL        string   `json:"archive_url"`       // Internet Archive URL
	ArchiveTimestamp  string   `json:"archive_timestamp"` // Internet Archive timestamp
	LastStatusChecked string   `json:"last_checked"`      // Last checked timestamp.
	HTTPStatusCode    int      `json:"status_code"`       // HTTP status code (200, 404, etc.)
	HTTPStatusText    string   `json:"status_text"`       // OK, Not Found, etc
	IsActive          bool     `json:"is_active"`         // true if the URL is active (200-299)
}

func (b *Bookmark) JSON() *BookmarkJSON {
	bj := &BookmarkJSON{}

	bVal := reflect.ValueOf(b).Elem()
	jsonVal := reflect.ValueOf(bj).Elem()

	for i := 0; i < bVal.NumField(); i++ {
		field := bVal.Type().Field(i)
		if field.Name == "Tags" {
			continue
		}

		jsonField := jsonVal.FieldByName(field.Name)
		if jsonField.IsValid() && jsonField.CanSet() {
			jsonField.Set(bVal.Field(i))
		}
	}

	t := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool {
			return r == ',' || r == ' '
		})
	}

	bj.Tags = t(b.Tags)

	return bj
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

// HashDomain returns the hash domain of a bookmark.
func (b *Bookmark) HashDomain() (string, error) {
	domain, err := b.Domain()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return hashDomain(domain), nil
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

func NewFromJSON(j *BookmarkJSON) *Bookmark {
	var b Bookmark
	data, _ := json.Marshal(j)
	_ = json.Unmarshal(data, &b)

	// convert tags back to string
	b.Tags = ParseTags(strings.Join(j.Tags, ","))

	return &b
}

func NewFromBuffer(buf []byte) (*Bookmark, error) {
	return fromBytes(buf)
}
