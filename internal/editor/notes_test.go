package editor

import (
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestNotesStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{"BuildBuffer", testBuildBuffer},
		{"ParseBuffer_Unchanged", testParseBufferUnchanged},
		{"ParseBuffer_Changed", testParseBufferChanged},
		{"Diff", testDiff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}

func testBuildBuffer(t *testing.T) {
	t.Helper()
	config.App.DBName = "test.db"
	config.App.Info.Version = "1.2.3"

	b := &bookmark.Bookmark{
		ID:    42,
		Title: "Example Title",
		URL:   "https://example.com/verylongurl",
		Notes: "Existing notes",
	}

	s := NotesStrategy{}
	buf, err := s.BuildBuffer(b, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(buf)

	expectedSubstrings := []string{
		"bookmark notes",
		"Example Title",
		"Existing notes",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in buffer, got:\n%s", expected, out)
		}
	}
}

func testParseBufferUnchanged(t *testing.T) {
	t.Helper()
	orig := &bookmark.Bookmark{Notes: "abc"}
	s := NotesStrategy{}
	buf := []byte("# Notes\nabc")

	_, err := s.ParseBuffer(buf, orig, 0, 1)
	if !errors.Is(err, ErrBufferUnchanged) {
		t.Errorf("expected ErrBufferUnchanged, got %v", err)
	}
}

func testParseBufferChanged(t *testing.T) {
	t.Helper()
	orig := &bookmark.Bookmark{Notes: "abc"}
	s := NotesStrategy{}
	buf := []byte("# Notes\nxyz")

	rec, err := s.ParseBuffer(buf, orig, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Notes != "xyz" {
		t.Errorf("expected updated notes to 'xyz', got %q", rec.Notes)
	}
}

func testDiff(t *testing.T) {
	t.Helper()
	oldB := &bookmark.Bookmark{Notes: "foo"}
	newB := &bookmark.Bookmark{Notes: "bar"}
	s := NotesStrategy{}

	diff := s.Diff(oldB, newB)
	if diff == "" {
		t.Errorf("expected non-empty diff for different notes")
	}
}
