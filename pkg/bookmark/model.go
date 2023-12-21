package bookmark

import (
	"bytes"
	"errors"
	"fmt"

	"gomarks/pkg/config"
	"gomarks/pkg/format"
)

var (
	// bookmarks
	ErrBookmarkURLEmpty    = errors.New("URL cannot be empty")
	ErrBookmarkTagsEmpty   = errors.New("tags cannot be empty")
	ErrBookmarkDuplicate   = errors.New("bookmark already exists")
	ErrBookmarkInvalid     = errors.New("bookmark invalid")
	ErrBookmarkNotSelected = errors.New("no bookmarks selected")
	ErrInvalidRecordID     = errors.New("invalid id")
)

type Bookmark struct {
	CreatedAt string `json:"created_at" db:"created_at"`
	URL       string `json:"url"        db:"url"`
	Tags      string `json:"tags"       db:"tags"`
	Title     string `json:"title"      db:"title"`
	Desc      string `json:"desc"       db:"desc"`
	ID        int    `json:"id"         db:"id"`
}

func (b *Bookmark) String() string {
	space := format.Space + format.Space + "+"
	maxLen := config.Term.MaxWidth - len(space) - len("\n")
	title := format.SplitAndAlignString(b.Title, maxLen)
	url := format.ShortenString(b.URL, maxLen)
	desc := format.SplitAndAlignString(b.Desc, maxLen)

	s := format.TitleLine(b.ID, format.Text(title).Purple().Bold().String())
	s += fmt.Sprintln(format.Text(space, url).Blue())
	s += fmt.Sprintln(format.Text(space, b.Tags).Gray().Bold())
	s += fmt.Sprintln(space, desc)
	return s
}

func (b *Bookmark) Update(url, title, tags, desc string) {
	b.URL = url
	b.Title = title
	b.Tags = format.ParseTags(tags)
	b.Desc = desc
}

func (b *Bookmark) Buffer() []byte {
	data := []byte(fmt.Sprintf(`## [%d] - %s
## url:
%s
## title: (leave an empty line for web fetch)
%s
## tags: (comma separated)
%s
## description: (leave an empty line for web fetch)
%s
## end
`, b.ID, b.Title, b.URL, b.Title, b.Tags, b.Desc))

	return bytes.TrimRight(data, " ")
}

func NewBookmark(url, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   url,
		Title: title,
		Tags:  format.ParseTags(tags),
		Desc:  desc,
	}
}
