package bookmark

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gomarks/pkg/color"
	"gomarks/pkg/util"

	"github.com/atotto/clipboard"
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
	CreatedAt string     `json:"created_at" db:"created_at"`
	URL       string     `json:"url"        db:"url"`
	Tags      string     `json:"tags"       db:"tags"`
	Title     NullString `json:"title"      db:"title"`
	Desc      NullString `json:"desc"       db:"desc"`
	ID        int        `json:"id"         db:"id"`
}

func (b *Bookmark) CopyToClipboard() {
	err := clipboard.WriteAll(b.URL)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}

	log.Print("Text copied to clipboard:", b.URL)
}

func (b *Bookmark) prettyString() string {
	// FIX: DRY
	maxLen := 80
	title := util.SplitAndAlignString(b.Title.String, maxLen)
	s := util.FormatTitleLine(b.ID, title, color.Purple)
	s += util.FormatLine("\t+ ", b.URL, color.Blue)
	s += util.FormatLine("\t+ ", b.Tags, color.Gray)

	if b.Desc.String != "" {
		desc := util.SplitAndAlignString(b.Desc.String, maxLen)
		s += util.FormatLine("\t+ ", desc, color.White)
	} else {
		s += util.FormatLine("\t+ ", "Untitled", color.White)
	}

	return s
}

func (b *Bookmark) PlainString() string {
	// FIX: DRY
	maxLen := 80
	title := util.SplitAndAlignString(b.Title.String, maxLen)
	s := util.FormatTitleLine(b.ID, title, "")
	s += util.FormatLine("\t+ ", b.URL, "")
	s += util.FormatLine("\t+ ", b.Tags, "")

	if b.Desc.String != "" {
		desc := util.SplitAndAlignString(b.Desc.String, maxLen)
		s += util.FormatLine("\t+ ", desc, "")
	}

	return s
}

func (b *Bookmark) String() string {
	return b.PlainString()
}

func (b *Bookmark) PrettyColorString() string {
	return b.prettyString()
}

func (b *Bookmark) setURL(url string) {
	b.URL = url
}

func (b *Bookmark) setTitle(title string) {
	b.Title.String = title
	b.Title.Valid = true
}

func (b *Bookmark) setDesc(desc string) {
	b.Desc.String = desc
	b.Desc.Valid = true
}

func (b *Bookmark) setTags(tags string) {
	words := strings.Fields(tags)
	strWithoutSpaces := strings.Join(words, "")
	b.Tags = strWithoutSpaces
}

func (b *Bookmark) Update(url, title, tags, desc string) {
	b.setURL(url)
	b.setTitle(title)
	b.setTags(tags)
	b.setDesc(desc)
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
`, b.ID, b.URL, b.URL, b.Title.String, b.Tags, b.Desc.String))

	return bytes.TrimRight(data, " ")
}

type NullString struct {
	sql.NullString
}

func (s NullString) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}

	data, err := json.Marshal(s.String)
	if err != nil {
		return nil, fmt.Errorf("error marshaling NullString: %w", err)
	}

	return data, nil
}

func (s *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		s.String, s.Valid = "", false
		return nil
	}

	s.String, s.Valid = string(data), true

	return nil
}

var InitBookmark = Bookmark{
	ID:    0,
	URL:   "https://github.com/haaag/GoMarks#readme",
	Title: NullString{NullString: sql.NullString{String: "GoMarks", Valid: true}},
	Tags:  "golang,awesome,bookmarks",
	Desc: NullString{
		sql.NullString{
			String: "Makes accessing, adding, updating, and removing bookmarks easier",
			Valid:  true,
		},
	},
}

func ToJSON(b *Slice) string {
	jsonData, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}

	jsonString := string(jsonData)

	return jsonString
}

func Create(url, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   url,
		Title: NullString{NullString: sql.NullString{String: title, Valid: true}},
		Tags:  parseTags(tags),
		Desc: NullString{
			sql.NullString{
				String: desc,
				Valid:  true,
			},
		},
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
