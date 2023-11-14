package bookmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/color"
	"gomarks/pkg/errs"
	"gomarks/pkg/format"
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

func NewSlice(b *Bookmark) *Slice {
	return &Slice{*b}
}

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
	url := format.ShortenString(b.URL, maxLen)
	title := format.SplitAndAlignString(b.Title, maxLen)
	desc := format.SplitAndAlignString(b.Desc, maxLen)

	s := format.FormatTitleLine(b.ID, title, color.Purple)
	s += format.FormatLine("\t+ ", url, color.Blue)
	s += format.FormatLine("\t+ ", b.Tags, color.Gray)
	s += format.FormatLine("\t+ ", desc, color.White)

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
		log.Print("bookmark is invalid. URL is empty")
		return false
	}

	if b.Tags == "," || b.Tags == "" {
		log.Print("bookmark is invalid. Tags are empty")
		return false
	}

	log.Print("bookmark is valid")

	return true
}

func (b *Bookmark) Buffer() []byte {
	data := []byte(fmt.Sprintf(`## editing [%d] %s
## lines starting with # will be ignored.
## url:
%s
## title: (leave an empty line for web fetch)
%s
## tags: (comma separated)
%s
## description: (leave an empty line for web fetch)
%s
## end
`, b.ID, b.URL, b.URL, b.Title, b.Tags, b.Desc))

	return bytes.TrimRight(data, " ")
}

func New(url, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   url,
		Title: title,
		Tags:  parseTags(tags),
		Desc:  desc,
	}
}

// convert: "tag1, tag2, tag3 tag"
// to: "tag1,tag2,tag3,tag,"
func parseTags(tags string) string {
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

func Format(f string, bs *Slice) error {
	switch f {
	case "json":
		j := ToJSON(bs)
		fmt.Println(j)
	case "pretty":
		for _, b := range *bs {
			fmt.Println(b.String())
		}
		total := color.Colorize(fmt.Sprintf("total [%d]", bs.Len()), color.Gray)
		fmt.Println(total)
	default:
		return fmt.Errorf("%w: %s", errs.ErrOptionInvalid, f)
	}

	return nil
}

func ToJSON(data interface{}) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}

	jsonString := string(jsonData)

	return jsonString
}
