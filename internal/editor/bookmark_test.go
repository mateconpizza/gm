package editor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestBookmarkStrategyParseBufferDelegates(t *testing.T) {
	t.Parallel()

	called := false
	fake := func(_ context.Context, _ []byte, _ *bookmark.Bookmark) (*bookmark.Bookmark, error) {
		called = true
		return &bookmark.Bookmark{ID: 99}, nil
	}

	bs := NewBookmarkStrategy().
		WithParseBuffer(fake)

	got, err := bs.ParseBuffer(t.Context(), nil, &bookmark.Bookmark{})
	if err != nil {
		t.Fatalf("ParseBuffer() unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("ParseBuffer() did not call injected parseBuffer")
	}
	if got.ID != 99 {
		t.Fatalf("ParseBuffer() = %+v; want injected result", got)
	}
}

func TestBookmarkStrategy_ParseBuffer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dummyBookmark := &bookmark.Bookmark{ID: 10}

	errCustom := errors.New("custom parse error")
	errEmptyBuffer := errors.New("empty buffer")

	tests := []struct {
		name     string
		setupFn  func() *BookmarkStrategy
		buf      []byte
		original *bookmark.Bookmark
		wantErr  error
	}{
		{
			name: "custom_parser_success",
			setupFn: func() *BookmarkStrategy {
				return NewBookmarkStrategy().
					WithParseBuffer(func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
						return &bookmark.Bookmark{ID: 999}, nil
					})
			},
			buf:      []byte("test buffer"),
			original: dummyBookmark,
			wantErr:  nil,
		},
		{
			name: "custom_parser_error",
			setupFn: func() *BookmarkStrategy {
				return NewBookmarkStrategy().
					WithParseBuffer(func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
						return nil, errCustom
					})
			},
			buf:      []byte("test buffer"),
			original: dummyBookmark,
			wantErr:  errCustom,
		},
		{
			name: "empty_buffer_handled_by_custom",
			setupFn: func() *BookmarkStrategy {
				return NewBookmarkStrategy().
					WithParseBuffer(func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
						if len(buf) == 0 {
							return nil, errEmptyBuffer
						}
						return original, nil
					})
			},
			buf:      []byte(""),
			original: dummyBookmark,
			wantErr:  errEmptyBuffer,
		},
		{
			name: "nil_buffer_handled_by_custom",
			setupFn: func() *BookmarkStrategy {
				return NewBookmarkStrategy().
					WithParseBuffer(func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
						if buf == nil {
							return nil, errEmptyBuffer
						}
						return original, nil
					})
			},
			buf:      nil,
			original: dummyBookmark,
			wantErr:  errEmptyBuffer,
		},
		{
			name: "nil_original_handled_by_custom",
			setupFn: func() *BookmarkStrategy {
				return NewBookmarkStrategy().
					WithParseBuffer(func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
						if original == nil {
							return nil, errCustom
						}
						return original, nil
					})
			},
			buf:      []byte("data"),
			original: nil,
			wantErr:  errCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bs := tt.setupFn()
			got, err := bs.ParseBuffer(ctx, tt.buf, tt.original)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ParseBuffer() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseBuffer() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseBuffer() unexpected error: %v", err)
			}

			if got == nil {
				t.Fatalf("ParseBuffer() returned nil bookmark for successful parse")
			}
		})
	}
}

func TestBookmarkStrategyZeroValueFallsBackToDefault(t *testing.T) {
	t.Parallel()

	var bs BookmarkStrategy // zero value, no NewBookmarkStrategy call

	original := testutil.NewBookmark(t)
	original.Tags = bookmark.ParseTags(original.Tags)
	_, err := bs.ParseBuffer(t.Context(), original.Buffer(), original)
	if !errors.Is(err, ErrBufferUnchanged) {
		t.Fatalf("zero-value BookmarkStrategy.ParseBuffer() = %v; want ErrBufferUnchanged (fallback to defaultParseBuffer)", err)
	}
}

func TestBookmarkStrategy_BuildBuffer(t *testing.T) {
	t.Parallel()

	bs := NewBookmarkStrategy()
	m := NewMeta("test_db", "1.2.3")

	padded := func(s, v any) string {
		return txt.PaddedLineWithPad(s, v, 10)
	}

	tests := []struct {
		name      string
		b         *bookmark.Bookmark
		idx       int
		total     int
		wantTexts []string
		notTexts  []string
	}{
		{
			name: "normal_existing_bookmark",
			b: &bookmark.Bookmark{
				ID:    10,
				URL:   "https://existing.com",
				Title: "Existing Bookmark",
			},
			idx:   3,
			total: 5,
			wantTexts: []string{
				" bookmark edition ",
				"[3/5]",
				"10 Existing Bookmark",
				padded("database:", m.dbName),
				"v1.2.3",
				"https://existing.com",
			},
			notTexts: []string{"[New]", " bookmark addition "},
		},
		{
			name: "new_bookmark_zero_id",
			b: &bookmark.Bookmark{
				ID:    0,
				URL:   "https://new.com",
				Title: "New Bookmark",
			},
			idx:   1,
			total: 1,
			wantTexts: []string{
				" bookmark addition ",
				"[New]",
				"0 New Bookmark",
				padded("database:", m.dbName),
				"https://new.com",
			},
			notTexts: []string{"[1/1]", " bookmark edition "},
		},
		{
			name: "title_with_newlines_boundary",
			b: &bookmark.Bookmark{
				ID:    42,
				URL:   "https://multiline.com",
				Title: "Title\nWith\nNewlines",
			},
			idx:   1,
			total: 10,
			wantTexts: []string{
				"42 Title With Newlines", // Verifies newline replacement logic in header
				"Title\nWith\nNewlines",
			},
			notTexts: nil,
		},
		{
			name:  "empty_bookmark_zero_values",
			b:     &bookmark.Bookmark{},
			idx:   0,
			total: 0,
			wantTexts: []string{
				" bookmark addition ",
				"[New]",
				padded("database:", m.dbName),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := bs.BuildBuffer(m, tt.b, tt.idx, tt.total)
			if err != nil {
				t.Fatalf("BuildBuffer() unexpected error: %v", err)
			}

			// Use strings.Contains instead of exact byte matching to avoid brittleness
			// caused by dynamic external dependencies like terminal.MinWidth().
			gotStr := string(got)
			for _, want := range tt.wantTexts {
				if !strings.Contains(gotStr, want) {
					t.Errorf("BuildBuffer() missing expected text %q\nOutput:\n%s", want, gotStr)
				}
			}

			for _, notWant := range tt.notTexts {
				if strings.Contains(gotStr, notWant) {
					t.Errorf("BuildBuffer() contains unexpected text %q", notWant)
				}
			}
		})
	}
}

func TestBookmarkFromBytes(t *testing.T) {
	t.Parallel()

	validB := testutil.NewBookmark(t)
	validBuffer := validB.Buffer()

	tests := []struct {
		name      string
		buf       []byte
		wantURL   string
		wantTitle string
	}{
		{
			name:      "normal_typical_input_from_buffer",
			buf:       validBuffer,
			wantURL:   "https://www.example.com",
			wantTitle: "Title",
		},
		{
			name:      "empty_bytes_edge_case",
			buf:       []byte(""),
			wantURL:   "",
			wantTitle: "",
		},
		{
			name:      "nil_buffer_edge_case",
			buf:       nil,
			wantURL:   "",
			wantTitle: "",
		},
		{
			name: "partial_content_boundary",
			buf: []byte(`# *URL:   (required)
https://partial.com
# Title:  (leave an empty line for web fetch)
Partial Title
# Tags:   (comma separated)`),
			wantURL:   "https://partial.com",
			wantTitle: "Partial Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := &bookmark.Bookmark{}
			bookmarkFromBytes(tt.buf, target)

			if target.URL != tt.wantURL {
				t.Errorf("bookmarkFromBytes() URL = %q; want %q", target.URL, tt.wantURL)
			}
			if target.Title != tt.wantTitle {
				t.Errorf("bookmarkFromBytes() Title = %q; want %q", target.Title, tt.wantTitle)
			}
		})
	}
}

func TestDefaultParseBuffer(t *testing.T) {
	t.Parallel()

	// editBuffer takes a real bookmark's own Buffer() output and swaps in new
	// field values, so tests exercise the exact frame format bookmarkFromBytes
	// parses in production.
	editBuffer := func(b *bookmark.Bookmark, url, title, tags, desc string) []byte {
		edited := b.Copy()
		edited.URL = url
		edited.Title = title
		edited.Tags = tags
		edited.Desc = desc
		return edited.Buffer()
	}

	tests := []struct {
		name       string
		original   *bookmark.Bookmark
		buf        []byte
		wantErr    error
		wantErrAny bool
	}{
		{
			name:     "normal_edit_changes_title_and_tags",
			original: testutil.NewBookmark(t),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "New Title", "new,tag", "Description"),
		},
		{
			name: "buffer_unchanged_returns_sentinel",
			original: func() *bookmark.Bookmark {
				b := testutil.NewBookmark(t)
				b.Tags = bookmark.ParseTags(b.Tags)
				return b
			}(),
			buf: func() []byte {
				b := testutil.NewBookmark(t)
				b.Tags = bookmark.ParseTags(b.Tags)
				return b.Buffer()
			}(),
			wantErr: ErrBufferUnchanged,
		},
		{
			name:     "empty_url_fails_validation",
			original: testutil.NewBookmark(t),
			buf: editBuffer(testutil.NewBookmark(t),
				"", "Title", "test,tag1,go", "Description"),
			wantErrAny: true,
		},
		{
			name:     "empty_tags_field",
			original: testutil.NewBookmark(t),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "Title", "", "Description"),
		},
		{
			name:     "multiline_title_collapsed",
			original: testutil.NewBookmark(t),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "Line one\nLine two", "test,tag1,go", "Description"),
		},
		{
			name: "new_bookmark_zero_id",
			original: func() *bookmark.Bookmark {
				b := testutil.NewBookmark(t)
				b.ID = 0
				return b
			}(),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "Final Title", "test,tag1,go", "Description"),
		},
		{
			name: "notes_preserved_regardless_of_buffer",
			original: func() *bookmark.Bookmark {
				b := testutil.NewBookmark(t)
				b.Notes = "important notes"
				return b
			}(),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "New Title", "test,tag1,go", "Description"),
		},
		{
			name:     "whitespace_only_description",
			original: testutil.NewBookmark(t),
			buf: editBuffer(testutil.NewBookmark(t),
				"https://www.example.com", "Title", "test,tag1,go", "   \n  "),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := defaultParseBuffer(t.Context(), tt.buf, tt.original)

			if tt.wantErrAny {
				if err == nil {
					t.Fatalf("defaultParseBuffer() expected an error, got nil")
				}
				return
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("defaultParseBuffer() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("defaultParseBuffer() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("defaultParseBuffer() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("defaultParseBuffer() returned nil bookmark with no error")
			}
			if got.Notes != tt.original.Notes {
				t.Fatalf("Notes = %q; want preserved original %q", got.Notes, tt.original.Notes)
			}
			if got == tt.original {
				t.Fatalf("defaultParseBuffer() returned the same pointer as original; want a copy")
			}
		})
	}
}
