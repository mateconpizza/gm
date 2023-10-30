package bookmark

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	"gomarks/pkg/color"
	u "gomarks/pkg/util"

	"github.com/atotto/clipboard"
)

type BookmarkSlice []Bookmark

// https://medium.com/@raymondhartoyo/one-simple-way-to-handle-null-database-value-in-golang-86437ec75089
type Bookmark struct {
	ID         int        `json:"id"         db:"id"`
	URL        string     `json:"url"        db:"url"`
	Title      NullString `json:"title"      db:"title"`
	Tags       string     `json:"tags"       db:"tags"`
	Desc       NullString `json:"desc"       db:"desc"`
	Created_at string     `json:"created_at" db:"created_at"`
}

func (b *Bookmark) CopyToClipboard() {
	err := clipboard.WriteAll(b.URL)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}
	log.Print("Text copied to clipboard:", b.URL)
}

func (b Bookmark) prettyString() string {
	// FIX: DRY
	maxLen := 80
	title := u.SplitAndAlignString(b.Title.String, maxLen)
	s := u.FormatTitleLine(b.ID, title, color.Purple)
	s += u.FormatLine("\t+ ", b.URL, color.Blue)
	s += u.FormatLine("\t+ ", b.Tags, color.Gray)
	if b.Desc.String != "" {
		desc := u.SplitAndAlignString(b.Desc.String, maxLen)
		s += u.FormatLine("\t+ ", desc, color.White)
	} else {
		s += u.FormatLine("\t+ ", "Untitled", color.White)
	}
	return s
}

func (b Bookmark) PlainString() string {
	// FIX: DRY
	maxLen := 80
	title := u.SplitAndAlignString(b.Title.String, maxLen)
	s := u.FormatTitleLine(b.ID, title, "")
	s += u.FormatLine("\t+ ", b.URL, "")
	s += u.FormatLine("\t+ ", b.Tags, "")
	if b.Desc.String != "" {
		desc := u.SplitAndAlignString(b.Desc.String, maxLen)
		s += u.FormatLine("\t+ ", desc, "")
	}
	return s
}

func (b Bookmark) String() string {
	return b.PlainString()
}

func (b Bookmark) PrettyColorString() string {
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

func (b Bookmark) IsValid() bool {
	if b.URL != "" {
		log.Print("IsValid: Bookmark is valid")
		return true
	}
	log.Print("IsValid: Bookmark is invalid")
	return false
}

type NullString struct {
	sql.NullString
}

func (s NullString) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(s.String)
}

func (s *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		s.String, s.Valid = "", false
		return nil
	}
	s.String, s.Valid = string(data), true
	return nil
}

var InitBookmark Bookmark = Bookmark{
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

func ToJSON(b *BookmarkSlice) string {
	jsonData, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	jsonString := string(jsonData)
	return jsonString
}
