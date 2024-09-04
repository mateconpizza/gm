package bookmark

import (
	"errors"
	"fmt"
)

var (
	ErrBookmarkURLEmpty    = errors.New("URL cannot be empty")
	ErrBookmarkTagsEmpty   = errors.New("tags cannot be empty")
	ErrBookmarkDuplicate   = errors.New("bookmark already exists")
	ErrBookmarkInvalid     = errors.New("bookmark invalid")
	ErrBookmarkNotSelected = errors.New("no bookmarks selected")
	ErrBookmarkNotFound    = errors.New("no bookmarks found")
	ErrInvalidRecordID     = errors.New("invalid id")
	ErrInvalidInput        = errors.New("invalid input")
)

// Bookmark represents a bookmark.
type Bookmark struct {
	CreatedAt string `db:"created_at" json:"created_at"`
	URL       string `db:"url"        json:"url"`
	Tags      string `db:"tags"       json:"tags"`
	Title     string `db:"title"      json:"title"`
	Desc      string `db:"desc"       json:"desc"`
	ID        int    `db:"id"         json:"id"`
}

func (b *Bookmark) GetID() int {
	return b.ID
}

func (b *Bookmark) GetURL() string {
	return b.URL
}

func (b *Bookmark) GetTags() string {
	return b.Tags
}

func (b *Bookmark) GetTitle() string {
	return b.Title
}

func (b *Bookmark) GetDesc() string {
	return b.Desc
}

func (b *Bookmark) GetCreatedAt() string {
	return b.CreatedAt
}

// Buffer returns a complete buf.
func (b *Bookmark) Buffer() []byte {
	return []byte(fmt.Sprintf(`# URL:
%s
# Title: (leave an empty line for web fetch)
%s
# Tags: (comma separated)
%s
# Description: (leave an empty line for web fetch)
%s
# end
`, b.GetURL(), b.GetTitle(), b.GetTags(), b.GetDesc()))
}

// BufSimple returns a simple buf with ID, title, tags and URL.
func (b *Bookmark) BufSimple() []byte {
	id := fmt.Sprintf("[%d]", b.ID)
	return []byte(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
}

// New creates a new bookmark.
func New(bURL, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   bURL,
		Title: title,
		Tags:  tags,
		Desc:  desc,
	}
}
