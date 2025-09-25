package editor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestJSONStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{"BuildBuffer", testJSONBuildBuffer},
		{"ParseBuffer_Unchanged", testJSONParseBufferUnchanged},
		{"ParseBuffer_Changed", testJSONParseBufferChanged},
		{"ParseBuffer_InvalidJSON", testJSONParseBufferInvalidJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}

func testJSONBuildBuffer(t *testing.T) {
	t.Helper()
	b := &bookmark.Bookmark{
		ID:    1,
		Title: "Test Title",
		URL:   "https://example.com",
		Notes: "Test notes",
	}

	s := JSONStrategy{}
	buf, err := s.BuildBuffer(b, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := b.Bytes()
	if !bytes.Equal(buf, expected) {
		t.Errorf("expected buffer to equal bookmark bytes\nexpected: %s\ngot: %s", expected, buf)
	}
}

func testJSONParseBufferUnchanged(t *testing.T) {
	t.Helper()
	original := &bookmark.Bookmark{
		ID:    1,
		Title: "Test",
		URL:   "https://example.com",
		Notes: "notes",
	}

	s := JSONStrategy{}
	buf := original.Bytes()

	_, err := s.ParseBuffer(buf, original, 0, 1)
	if !errors.Is(err, parser.ErrBufferUnchanged) {
		t.Errorf("expected parser.ErrBufferUnchanged, got %v", err)
	}
}

func testJSONParseBufferChanged(t *testing.T) {
	t.Helper()
	original := &bookmark.Bookmark{
		ID:    1,
		Title: "Original Title",
		URL:   "https://example.com",
		Notes: "original notes",
	}

	modified := &bookmark.Bookmark{
		ID:    1,
		Title: "Modified Title",
		URL:   "https://example.com",
		Notes: "modified notes",
	}

	s := JSONStrategy{}
	buf := modified.Bytes()

	result, err := s.ParseBuffer(buf, original, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != "Modified Title" {
		t.Errorf("expected title 'Modified Title', got %q", result.Title)
	}
	if result.Notes != "modified notes" {
		t.Errorf("expected notes 'modified notes', got %q", result.Notes)
	}
}

func testJSONParseBufferInvalidJSON(t *testing.T) {
	t.Helper()
	original := &bookmark.Bookmark{
		ID:    1,
		Title: "Test",
		URL:   "https://example.com",
	}

	s := JSONStrategy{}
	invalidBuf := []byte("{invalid json")

	_, err := s.ParseBuffer(invalidBuf, original, 0, 1)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
