package bookmark

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrDuplicate    = errors.New("bookmark already exists")
	ErrInvalid      = errors.New("bookmark invalid")
	ErrInvalidID    = errors.New("invalid bookmark id")
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("no bookmark found")
	ErrNotSelected  = errors.New("no bookmark selected")
	ErrTagsEmpty    = errors.New("TAGS cannot be empty")
	ErrURLEmpty     = errors.New("URL cannot be empty")
	ErrUnknownField = errors.New("bookmark field unknown")
)

// Bookmark represents a bookmark.
type Bookmark struct {
	URL        string `db:"url"         json:"url"         yaml:"url"`
	Tags       string `db:"tags"        json:"tags"        yaml:"-"`
	Title      string `db:"title"       json:"title"       yaml:"title"`
	Desc       string `db:"desc"        json:"desc"        yaml:"desc"`
	ID         int    `db:"id"          json:"id"          yaml:"id"`
	CreatedAt  string `db:"created_at"  json:"created_at"  yaml:"created_at"`
	LastVisit  string `db:"last_visit"  json:"last_visit"  yaml:"last_visit"`
	UpdatedAt  string `db:"updated_at"  json:"updated_at"  yaml:"updated_at"`
	VisitCount int    `db:"visit_count" json:"visit_count" yaml:"visit_count"`
	Favorite   bool   `db:"favorite"    json:"favorite"    yaml:"favorite"`
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
# Description: (leave an empty line for web fetch)
%s
# end ------------------------------------------------------------------`,
		b.URL, b.Title, ParseTags(b.Tags), b.Desc)
}

// New creates a new bookmark.
func New() *Bookmark {
	return &Bookmark{}
}
