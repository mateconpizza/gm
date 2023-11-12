package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/color"
	"gomarks/pkg/util"
)

type Slice []Bookmark

func (bs *Slice) Len() int {
	return len(*bs)
}

func (bs *Slice) Get(index int) *Bookmark {
	if index >= 0 && index < bs.Len() {
		return &(*bs)[index]
	}
	return nil
}

func (bs *Slice) IDs() []int {
	ids := make([]int, 0, bs.Len())
	for _, b := range *bs {
		ids = append(ids, b.ID)
	}
	return ids
}

// https://medium.com/@raymondhartoyo/one-simple-way-to-handle-null-database-value-in-golang-86437ec75089
type Bookmark struct {
	CreatedAt string `json:"created_at" db:"created_at"`
	URL       string `json:"url"        db:"url"`
	Tags      string `json:"tags"       db:"tags"`
	Title     string `json:"title"      db:"title"`
	Desc      string `json:"desc"       db:"desc"`
	ID        int    `json:"id"         db:"id"`
}

func (b *Bookmark) prettyString() string {
	maxLen := 80
	url := util.ShortenString(b.URL, maxLen)
	title := util.SplitAndAlignString(b.Title, maxLen)
	s := util.FormatTitleLine(b.ID, title, color.Purple)
	s += util.FormatLine("\t+ ", url, color.Blue)
	s += util.FormatLine("\t+ ", b.Tags, color.Gray)

	if b.Desc != "" {
		desc := util.SplitAndAlignString(b.Desc, maxLen)
		s += util.FormatLine("\t+ ", desc, color.White)
	} else {
		s += util.FormatLine("\t+ ", "Untitled", color.White)
	}

	return s
}

func (b *Bookmark) String() string {
	return b.prettyString()
}

func (b *Bookmark) Update(url, title, tags, desc string) {
	b.URL = url
	b.Title = title
	b.Tags = parseTags(tags)
	b.Desc = desc
}

func (b *Bookmark) IsValid() bool {
	if b.URL == "" {
		log.Print("Bookmark is invalid. URL is empty")
		return false
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("Bookmark is invalid. Tags are empty")
		return false
	}

	log.Print("Bookmark is valid")

	return true
}

func (b *Bookmark) Buffer() []byte {
	data := []byte(fmt.Sprintf(`## editing [%d] %s
## lines starting with # will be ignored.
## url:
%s
## title: (leave empty line for web fetch)
%s
## tags: (comma separated)
%s
## description: (leave empty line for web fetch)
%s
## end
`, b.ID, b.URL, b.URL, b.Title, b.Tags, b.Desc))

	return bytes.TrimRight(data, " ")
}

var InitBookmark = Bookmark{
	ID:    0,
	URL:   "https://github.com/haaag/GoMarks#readme",
	Title: "Gomarks",
	Tags:  "golang,awesome,bookmarks",
	Desc:  "Makes accessing, adding, updating, and removing bookmarks easier",
}

func Create(url, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   url,
		Title: title,
		Tags:  parseTags(tags),
		Desc:  desc,
	}
}

// convert: "tag1, tag3, tag tag"
// to:      "tag1,tag3,tag,tag,"
func parseTags(tags string) string {
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}
