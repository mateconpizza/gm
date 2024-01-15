package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/terminal"

	"github.com/atotto/clipboard"
)

var (
	// bookmarks
	ErrBookmarkURLEmpty    = errors.New("URL cannot be empty")
	ErrBookmarkTagsEmpty   = errors.New("tags cannot be empty")
	ErrBookmarkDuplicate   = errors.New("bookmark already exists")
	ErrBookmarkInvalid     = errors.New("bookmark invalid")
	ErrBookmarkNotSelected = errors.New("no bookmarks selected")
	ErrBookmarkNotFound    = errors.New("no bookmarks found")
	ErrInvalidRecordID     = errors.New("invalid id")
	ErrInvalidInput        = errors.New("invalid input")
	ErrCopyToClipboard     = errors.New("copy to clipboard")
)

type Slice []Bookmark

func (s Slice) Remove(idx int) Slice {
	return append(s[:idx], s[idx+1:]...)
}

func (s *Slice) Append(b *Bookmark) {
	*s = append(*s, *b)
}

func (s *Slice) Index(id int) int {
	for i, b := range *s {
		if b.ID == id {
			return i
		}
	}
	return -1
}

func NewSlice() Slice {
	return make([]Bookmark, 0)
}

type Bookmark struct {
	CreatedAt string `json:"created_at" db:"created_at"`
	URL       string `json:"url"        db:"url"`
	Tags      string `json:"tags"       db:"tags"`
	Title     string `json:"title"      db:"title"`
	Desc      string `json:"desc"       db:"desc"`
	ID        int    `json:"id"         db:"id"`
}

func (b *Bookmark) String() string {
	sep := strings.Repeat(format.Separator, 6) + "+"
	maxLen := terminal.Settings.MaxWidth - len(sep) - len("\n")
	title := format.SplitAndAlignString(b.Title, maxLen)
	url := format.ShortenString(b.URL, maxLen)
	desc := format.SplitAndAlignString(b.Desc, maxLen)

	sb := strings.Builder{}

	sb.WriteString(format.HeaderLine(b.ID, format.Text(title).Purple().Bold().String()))
	sb.WriteString(format.Text(sep, url, "\n").Blue().String())
	sb.WriteString(format.Text(sep, b.Tags, "\n").Gray().Bold().String())
	sb.WriteString(format.Text(sep, desc, "\n").String())
	return sb.String()
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

func (b *Bookmark) Edit(r *SQLiteRepository) error {
	editedContent, err := Edit(b.Buffer())
	if err != nil {
		if errors.Is(err, ErrBufferUnchanged) {
			fmt.Printf("%s: bookmark [%d]: %s\n", config.App.Name, b.ID, format.Text("unchanged").Yellow().Bold())
			return nil
		}
		return fmt.Errorf("error editing bookmark: %w", err)
	}

	tempContent := strings.Split(string(editedContent), "\n")
	if err := ValidateBookmarkBuffer(tempContent); err != nil {
		return fmt.Errorf("%w", err)
	}

	tb := ParseTempBookmark(tempContent)
	FetchTitleAndDescription(tb)
	b.Update(tb.URL, tb.Title, tb.Tags, tb.Desc)

	if _, err := r.Update(config.DB.Table.Main, b); err != nil {
		return fmt.Errorf("error updating record: %w", err)
	}

	fmt.Printf("%s: bookmark [%d]: %s\n", config.App.Name, b.ID, format.Text("updated").Blue().Bold())
	return nil
}

func (b *Bookmark) Copy() error {
	err := clipboard.WriteAll(b.URL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	log.Print("text copied to clipboard:", b.URL)
	return nil
}

func (b *Bookmark) Open() error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", b.URL).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", b.URL).Start()
	case "darwin":
		err = exec.Command("open", b.URL).Start()
	default:
		err = terminal.ErrUnsupportedPlatform
	}
	if err != nil {
		return err
	}

	return nil
}

func NewBookmark(url, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   url,
		Title: title,
		Tags:  format.ParseTags(tags),
		Desc:  desc,
	}
}
