package bookmark

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/locker/gpg"
)

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
	ID         int    `db:"id"          json:"id"          yaml:"id"`
	URL        string `db:"url"         json:"url"         yaml:"url"`
	Tags       string `db:"tags"        json:"-"           yaml:"-"`
	Title      string `db:"title"       json:"title"       yaml:"title"`
	Desc       string `db:"desc"        json:"desc"        yaml:"desc"`
	CreatedAt  string `db:"created_at"  json:"created_at"  yaml:"created_at"`
	LastVisit  string `db:"last_visit"  json:"last_visit"  yaml:"last_visit"`
	UpdatedAt  string `db:"updated_at"  json:"updated_at"  yaml:"updated_at"`
	VisitCount int    `db:"visit_count" json:"visit_count" yaml:"visit_count"`
	Favorite   bool   `db:"favorite"    json:"favorite"    yaml:"favorite"`
	Checksum   string `db:"checksum"    json:"checksum"    yaml:"checksum"`
}

type BookmarkJSON struct {
	ID         int      `json:"id"`
	URL        string   `json:"url"`
	Tags       []string `json:"tags"`
	Title      string   `json:"title"`
	Desc       string   `json:"desc"`
	CreatedAt  string   `json:"created_at"`
	LastVisit  string   `json:"last_visit"`
	UpdatedAt  string   `json:"updated_at"`
	VisitCount int      `json:"visit_count"`
	Favorite   bool     `json:"favorite"`
	Checksum   string   `json:"checksum"`
}

func (b *Bookmark) ToJSON() *BookmarkJSON {
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
		Checksum:   b.Checksum,
	}
}

// Field returns the value of a field.
func (b *Bookmark) Field(f string) (string, error) {
	var field string
	var s string
	switch f {
	case "id", "i", "1":
		field = "id"
		s = strconv.Itoa(b.ID)
	case "url", "u", "2":
		field = "url"
		s = b.URL
	case "title", "t", "3":
		field = "title"
		s = b.Title
	case "tags", "T", "4":
		field = "tags"
		s = b.Tags
	case "desc", "d", "5":
		field = "desc"
		s = b.Desc
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownField, f)
	}

	slog.Info("selected field", "field", field)

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

func (b *Bookmark) GenerateChecksum() {
	b.Checksum = Checksum(b.URL, b.Title, b.Desc, b.Tags)
}

// HashPath returns the hash path of a bookmark.
//
//	hashDomain + Checksum
func (b *Bookmark) HashPath() (string, error) {
	domain, err := hashDomain(b.URL)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return filepath.Join(domain, b.Checksum), nil
}

// Domain returns the domain of a bookmark.
func (b *Bookmark) Domain() (string, error) {
	return domain(b.URL)
}

func (b *Bookmark) HashURL() string {
	return hashURL(b.URL)
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
func (b *Bookmark) GPGPath() (string, error) {
	domain, err := hashDomain(b.URL)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return filepath.Join(domain, b.Checksum+gpg.Extension), nil
}

// New creates a new bookmark.
func New() *Bookmark {
	return &Bookmark{}
}

func NewFromJSON(json *BookmarkJSON) *Bookmark {
	b := New()
	b.ID = json.ID
	b.URL = json.URL
	b.Title = json.Title
	b.Desc = json.Desc
	b.Tags = ParseTags(strings.Join(json.Tags, ","))
	b.CreatedAt = json.CreatedAt
	b.LastVisit = json.LastVisit
	b.UpdatedAt = json.UpdatedAt
	b.VisitCount = json.VisitCount
	b.Favorite = json.Favorite
	b.Checksum = json.Checksum
	return b
}
