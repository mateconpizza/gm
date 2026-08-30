package bookio

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestHTMLParse(t *testing.T) {
	t.Parallel()
	want := 10
	bs := testutil.NewBookmarkSlice(t, want)

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
