package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/url"
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
	space := 6
	indentation := 8
	sep := strings.Repeat(format.Space, space) + "+"
	maxLen := terminal.Settings.MaxWidth - len(sep) - len("\n")
	title := format.SplitAndAlignString(b.Title, maxLen, indentation)
	bURL := format.ShortenString(b.URL, maxLen)
	desc := format.SplitAndAlignString(b.Desc, maxLen, indentation)

	sb := strings.Builder{}
	sb.WriteString(format.HeaderLine(b.ID, format.Text(title).Purple().Bold().String()))
	sb.WriteString(format.Text(sep, bURL, "\n").Blue().String())
	sb.WriteString(format.Text(sep, b.Tags, "\n").Gray().Bold().String())
	sb.WriteString(format.Text(sep, desc, "\n").String())
	return sb.String()
}

func (b *Bookmark) BetaString() string {
	space := 6
	indentation := 8
	sep := strings.Repeat(format.Space, space) + "+"
	maxLen := terminal.Settings.MaxWidth - len(sep) - len("\n")
	title := format.SplitAndAlignString(b.Title, maxLen, indentation)
	prettyURL := PrettifyURL(b.URL)
	bURL := format.ShortenString(prettyURL, maxLen)
	desc := format.SplitAndAlignString(b.Desc, maxLen, indentation)

	sb := strings.Builder{}
	sb.WriteString(format.HeaderLine(b.ID, format.Text(bURL).Purple().String()))
	sb.WriteString(format.Text(sep, title, "\n").Blue().String())
	sb.WriteString(format.Text(sep, prettifyTags(b.Tags), "\n").Gray().Bold().String())
	sb.WriteString(format.Text(sep, desc, "\n").String())
	return sb.String()
}

func (b *Bookmark) DeleteString() string {
	space := 6
	indentation := 8
	sep := strings.Repeat(format.Space, space) + "+"
	maxLen := terminal.Settings.MaxWidth - len(sep) - len("\n")
	title := format.SplitAndAlignString(b.Title, maxLen, indentation)
	prettyURL := PrettifyURL(b.URL)
	bURL := format.ShortenString(prettyURL, maxLen)

	sb := strings.Builder{}
	sb.WriteString(format.HeaderLine(b.ID, format.Text(bURL).Red().Bold().String()))
	sb.WriteString(format.Text(sep, title, "\n").Blue().String())
	sb.WriteString(format.Text(sep, prettifyTags(b.Tags), "\n").Gray().Bold().String())
	return sb.String()
}

func prettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", format.BulletPoint)
	return strings.TrimRight(t, format.BulletPoint)
}

func (b *Bookmark) Update(bURL, title, tags, desc string) {
	b.URL = bURL
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
			unchanged := format.Text("unchanged").Yellow().Bold()
			fmt.Printf("%s: bookmark [%d]: %s\n", config.App.Name, b.ID, unchanged)
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

func NewBookmark(bURL, title, tags, desc string) *Bookmark {
	return &Bookmark{
		URL:   bURL,
		Title: title,
		Tags:  format.ParseTags(tags),
		Desc:  desc,
	}
}

func PrettifyURL(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return format.Text(bURL).Bold().String()
	}

	host := format.Text(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(strings.TrimLeft(u.Path, "/"), func(r rune) bool { return r == '/' })

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := format.Text(format.BulletPoint, strings.Join(pathSegments, fmt.Sprintf(" %s ", format.BulletPoint))).Gray()
	return fmt.Sprintf("%s %s", host, pathSeg)
}
