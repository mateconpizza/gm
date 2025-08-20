package bookio

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func testSingleBookmark() *bookmark.Bookmark {
	return &bookmark.Bookmark{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01T12:00:00Z",
		LastVisit: "2023-01-01T12:00:00Z",
		Favorite:  true,
		Checksum:  "checksum",
	}
}

func testSliceBookmarks(n int) []*bookmark.Bookmark {
	bs := make([]*bookmark.Bookmark, 0, n)
	for i := range n {
		b := testSingleBookmark()
		b.ID = i + 1
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)
		bs = append(bs, b)
	}

	return bs
}

func TestHTMLParse(t *testing.T) {
	t.Parallel()
	want := 10
	bs := testSliceBookmarks(want)

	if len(bs) != want {
		t.Fatal("unexpected number of bookmarks.")
	}

	var buf bytes.Buffer
	if err := ExportToNetscapeHTML(bs, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bp := NewHTMLParser()
	books, err := bp.ParseHTML(&buf)
	if err != nil {
		t.Fatalf("unexpected err parsing HTML: %v", err)
	}
	if len(books) != want {
		t.Fatalf("expected %d bookmarks, got %d", want, len(books))
	}

	// convert
	converted := make([]*bookmark.Bookmark, 0, len(books))
	for i := range books {
		converted = append(converted, FromNetscape(&books[i]))
	}
	if len(converted) != want {
		t.Fatalf("expected bookmarks %d, got %d", want, len(converted))
	}

	b := bs[0]
	var found *bookmark.Bookmark
	for i := range converted {
		if converted[i].Title == b.Title {
			found = converted[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected to found bookmark: %v", b)
	}
}

func TestExportHTML(t *testing.T) {
	t.Parallel()

	validFileContent := `
<!DOCTYPE NETSCAPE-Bookmark-file-1>
<H1>Bookmarks</H1>
`
	validBuf := bytes.NewBufferString(validFileContent)
	reader := bytes.NewReader(validBuf.Bytes())
	if err := IsValidNetscapeFile(reader); err != nil {
		t.Fatalf("unexpected error on validating: %v", err)
	}

	invalidFileContent := `
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
`
	invalidBuf := bytes.NewBufferString(invalidFileContent)
	err := IsValidNetscapeFile(bytes.NewReader(invalidBuf.Bytes()))
	if err == nil {
		t.Fatalf("expected error on validating: %v", err)
	}

	if !errors.Is(err, ErrNoNetscapeFile) {
		t.Fatalf("expected %v, got %v", ErrNoNetscapeFile, err)
	}
}
