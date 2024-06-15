package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/haaag/gm/pkg/scraper"
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

// Bookmark represents a bookmark
type Bookmark struct {
	CreatedAt string `json:"created_at" db:"created_at"`
	URL       string `json:"url"        db:"url"`
	Tags      string `json:"tags"       db:"tags"`
	Title     string `json:"title"      db:"title"`
	Desc      string `json:"desc"       db:"desc"`
	ID        int    `json:"id"         db:"id"`
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

func (b *Bookmark) SetID(i int) {
	b.ID = i
}

func (b *Bookmark) SetURL(s string) {
	b.URL = s
}

func (b *Bookmark) SetTitle(s string) {
	b.Title = s
}

func (b *Bookmark) SetTags(s string) {
	b.Tags = s
}

func (b *Bookmark) SetDesc(s string) {
	b.Desc = s
}

// Buffer returns a complete buf
func (b *Bookmark) Buffer() []byte {
	buf := bytes.NewBuffer([]byte{})
	fmt.Fprintf(buf, "# ID: [%d] (%s)\n", b.ID, b.GetCreatedAt())
	fmt.Fprintf(buf, `# URL:
%s
# Title: (leave an empty line for web fetch)
%s
# Tags: (comma separated)
%s
# Description: (leave an empty line for web fetch)
%s
# end`, b.URL, b.Title, b.Tags, b.Desc)
	return buf.Bytes()
}

// BufSimple returns s simple buf
func (b *Bookmark) BufSimple() []byte {
	id := fmt.Sprintf("[%d]", b.ID)
	return []byte(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
}

// New creates a new bookmark
func New(bURL, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   bURL,
		Title: title,
		Tags:  tags,
		Desc:  desc,
	}
}

// GetBufferSlice returns a buffer with the provided slice of bookmarks
func GetBufferSlice(bs *Slice[Bookmark]) []byte {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("## To keep a bookmark, remove the <URL> line\n")
	fmt.Fprintf(buf, "## Showing %d bookmark/s\n\n", bs.Len())
	bs.ForEach(func(b Bookmark) {
		buf.Write(b.BufSimple())
	})
	return bytes.TrimSpace(buf.Bytes())
}

// Validate validates the bookmark
func Validate(b *Bookmark) error {
	if b.URL == "" {
		log.Print("bookmark is invalid. URL is empty")
		return ErrBookmarkURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("bookmark is invalid. Tags are empty")
		return ErrBookmarkTagsEmpty
	}

	log.Print("bookmark is valid")
	return nil
}

func checkTitleAndDesc(b *Bookmark) {
	sc := scraper.New(b.URL)
	update := b.Title == "" || b.Desc == ""

	if update {
		_ = sc.Scrape()
		if b.Title == "" {
			b.Title = strings.TrimSpace(sc.Title)
		}
		if b.Desc == "" {
			b.Desc = strings.TrimSpace(sc.Desc)
		}
	}
}
