// Package bookmark contains the bookmark record.
package bookmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const DefaultTag = "notag"

var (
	ErrBookmarkDuplicate       = errors.New("bookmark already exists")
	ErrBookmarkInvalidID       = errors.New("invalid bookmark id")
	ErrBookmarkNotFound        = errors.New("no bookmark found")
	ErrBookmarkTagsEmpty       = errors.New("tags cannot be empty")
	ErrBookmarkURLEmpty        = errors.New("URL cannot be empty")
	ErrBookmarkTitleRequired   = errors.New("title is required")
	ErrBookmarkUnknownField    = errors.New("bookmark field unknown")
	ErrBookmarkInvalidChecksum = errors.New("invalid checksum")
	ErrBookmarkArchiveURL      = errors.New("no archive URL found")
)

// Bookmark represents a bookmark.
type Bookmark struct {
	// Identity
	ID int `db:"id" json:"id"`

	// Content (user-editable)
	URL   string `db:"url"   json:"url"`
	Title string `db:"title" json:"title"`
	Desc  string `db:"desc"  json:"desc"`
	Tags  string `db:"tags"  json:"tags"` // comma-separated tags
	Notes string `db:"notes" json:"notes"`

	// Timestamps
	CreatedAt         string `db:"created_at"   json:"created_at"`
	UpdatedAt         string `db:"updated_at"   json:"updated_at"`
	LastVisit         string `db:"last_visit"   json:"last_visit"`
	LastStatusChecked string `db:"last_checked" json:"last_checked"`

	// Usage stats
	VisitCount int  `db:"visit_count" json:"visit_count"`
	Favorite   bool `db:"favorite"    json:"favorite"`

	// Link health
	HTTPStatusCode int    `db:"status_code" json:"status_code"`
	HTTPStatusText string `db:"status_text" json:"status_text"` // OK, Not Found, etc
	IsActive       bool   `db:"is_active"   json:"is_active"`   // true if the URL is active (200-299)

	// Media / enrichment
	FaviconURL   string `db:"favicon_url"   json:"favicon_url"`   // URL for the bookmark's favicon.
	FaviconLocal string `db:"favicon_local" json:"favicon_local"` // Local path to the cached favicon file.

	// Archive metadata
	ArchiveURL       string `db:"archive_url"       json:"archive_url"`       // Internet Archive URL
	ArchiveTimestamp string `db:"archive_timestamp" json:"archive_timestamp"` // Internet Archive timestamp

	// Integrity
	Checksum string `db:"checksum" json:"checksum"` // Checksum or hash (URL, Title, Description and Tags)
}

// New creates a new bookmark.
func New() *Bookmark { return &Bookmark{} }

func (b *Bookmark) Bytes() []byte {
	bj, err := json.MarshalIndent(b.Record(), "", "  ")
	if err != nil {
		return nil
	}

	return bj
}

// Equals reports whether b and o have the same URL, Tags, Title and Desc.
func (b *Bookmark) Equals(o *Bookmark) bool {
	if b == nil || o == nil {
		return b == o
	}

	return b.URL == o.URL &&
		b.Tags == o.Tags &&
		b.Title == o.Title &&
		b.Desc == o.Desc &&
		b.Notes == o.Notes
}

func (b *Bookmark) DeepEquals(o *Bookmark) bool {
	if b == nil || o == nil {
		return b == o
	}

	if b.Checksum == o.Checksum {
		return true
	}

	return *b == *o
}

func (b *Bookmark) Buffer() []byte {
	return fmt.Appendf(nil, `# *URL:   (required)
%s
# Title:  (leave an empty line for web fetch)
%s
# Tags:   (comma separated)
%s
# Description:
%s
# end ------------------------------------------------------------------`,
		b.URL, b.Title, ParseTags(b.Tags), b.Desc)
}

func (b *Bookmark) BufferNotes() []byte {
	return fmt.Appendf(nil, `# Notes: (lines starting with # will be ignored)
%s
`, b.Notes)
}

// GenChecksum generates a checksum for the bookmark.
//
// It uses the URL, Title, Description and Tags.
func (b *Bookmark) GenChecksum() { b.Checksum = genChecksum(b.URL, b.Title, b.Desc, b.Tags, b.Notes) }

// HashURL returns the hash of a bookmark URL.
func (b *Bookmark) HashURL() string { return hashURL(b.URL) }

// HashPath returns the hash path of a bookmark.
//
//	hashDomain + Checksum
func (b *Bookmark) HashPath() (string, error) {
	s, err := Domain(b.URL)
	if err != nil {
		return "", err
	}
	return filepath.Join(hashDomain(s), b.Checksum), nil
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
func (b *Bookmark) Domain() (string, error) { return Domain(b.URL) }

// Path returns the file path for the bookmark with the given extension.
//
//	domain -> urlHash.ext
func (b *Bookmark) Path(ext string) (string, error) {
	domain, err := Domain(b.URL)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	urlHash := hashURL(b.URL)
	return filepath.Join(domain, urlHash+ext), nil
}

// GPGPath returns the path to the GPG-encrypted bookmark file.
//
//	domainHash -> urlHash.gpg
func (b *Bookmark) GPGPath() (string, error) {
	hashPath, err := b.HashPath()
	if err != nil {
		return "", fmt.Errorf("hashing path: %w", err)
	}
	return hashPath + ".gpg", nil
}

func (b *Bookmark) Copy() *Bookmark {
	if b == nil {
		return nil
	}
	cp := *b
	return &cp
}

// Record converts the bookmark to its portable form.
func (b *Bookmark) Record() *Record {
	splitTags := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	}

	return &Record{
		ID:                b.ID,
		URL:               b.URL,
		Tags:              splitTags(b.Tags),
		Title:             b.Title,
		Desc:              b.Desc,
		Notes:             b.Notes,
		CreatedAt:         b.CreatedAt,
		LastVisit:         b.LastVisit,
		UpdatedAt:         b.UpdatedAt,
		VisitCount:        b.VisitCount,
		Favorite:          b.Favorite,
		FaviconURL:        b.FaviconURL,
		FaviconLocal:      b.FaviconLocal,
		Checksum:          b.Checksum,
		ArchiveURL:        b.ArchiveURL,
		ArchiveTimestamp:  b.ArchiveTimestamp,
		LastStatusChecked: b.LastStatusChecked,
		HTTPStatusCode:    b.HTTPStatusCode,
		HTTPStatusText:    b.HTTPStatusText,
		IsActive:          b.IsActive,
	}
}

func (b *Bookmark) String() string {
	return fmt.Sprintf(
		"[%d] %s - %s %s (visits: %d, favorite: %t, status: %d %s, active: %t)",
		b.ID,
		b.Title,
		b.URL,
		strings.Split(strings.TrimSuffix(b.Tags, ","), ","),
		b.VisitCount,
		b.Favorite,
		b.HTTPStatusCode,
		b.HTTPStatusText,
		b.IsActive,
	)
}

// Record is the portable form of a Bookmark, used for JSON and YAML.
type Record struct {
	ID                int      `json:"id"                yaml:"id"`
	URL               string   `json:"url"               yaml:"url"`
	Tags              []string `json:"tags"              yaml:"tags"`
	Title             string   `json:"title"             yaml:"title"`
	Desc              string   `json:"desc"              yaml:"desc"`
	Notes             string   `json:"notes"             yaml:"notes"`
	CreatedAt         string   `json:"created_at"        yaml:"created_at"`
	LastVisit         string   `json:"last_visit"        yaml:"last_visit,omitempty"`
	UpdatedAt         string   `json:"updated_at"        yaml:"updated_at"`
	VisitCount        int      `json:"visit_count"       yaml:"visit_count"`
	Favorite          bool     `json:"favorite"          yaml:"favorite"`
	FaviconURL        string   `json:"favicon_url"       yaml:"favicon_url,omitempty"`
	FaviconLocal      string   `json:"favicon_local"     yaml:"favicon_local,omitempty"`
	Checksum          string   `json:"checksum"          yaml:"checksum"`
	ArchiveURL        string   `json:"archive_url"       yaml:"archive_url,omitempty"`
	ArchiveTimestamp  string   `json:"archive_timestamp" yaml:"archive_timestamp,omitempty"`
	LastStatusChecked string   `json:"last_checked"      yaml:"last_checked,omitempty"`
	HTTPStatusCode    int      `json:"status_code"       yaml:"status_code"`
	HTTPStatusText    string   `json:"status_text"       yaml:"status_text"`
	IsActive          bool     `json:"is_active"         yaml:"is_active"`
}

func NewRecord() *Record { return &Record{} }

// Bookmark converts the record back to a Bookmark.
func (r *Record) Bookmark() *Bookmark {
	return &Bookmark{
		ID:                r.ID,
		URL:               r.URL,
		Tags:              ParseTags(strings.Join(r.Tags, ",")),
		Title:             r.Title,
		Desc:              r.Desc,
		Notes:             r.Notes,
		CreatedAt:         r.CreatedAt,
		LastVisit:         r.LastVisit,
		UpdatedAt:         r.UpdatedAt,
		VisitCount:        r.VisitCount,
		Favorite:          r.Favorite,
		FaviconURL:        r.FaviconURL,
		FaviconLocal:      r.FaviconLocal,
		Checksum:          r.Checksum,
		ArchiveURL:        r.ArchiveURL,
		ArchiveTimestamp:  r.ArchiveTimestamp,
		LastStatusChecked: r.LastStatusChecked,
		HTTPStatusCode:    r.HTTPStatusCode,
		HTTPStatusText:    r.HTTPStatusText,
		IsActive:          r.IsActive,
	}
}
