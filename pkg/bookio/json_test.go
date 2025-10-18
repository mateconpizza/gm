package bookio

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

func TestSaveAsJSON_CreatesFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := &bookmark.Bookmark{
		URL:      "https://example.com/page",
		Title:    "Example Page",
		Checksum: "abc123",
	}

	updated, err := SaveAsJSON(root, b, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected updated=true for new file")
	}

	domain, _ := b.Domain()
	urlHash := b.HashURL()
	path := filepath.Join(root, domain, urlHash+".json")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s, got %v", path, err)
	}
}

func TestSaveAsJSON_ConflictChecksumUpdate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// create initial
	b1 := &bookmark.Bookmark{
		URL:      "https://example.com/update",
		Title:    "Old Title",
		Checksum: "old123",
	}
	if _, err := SaveAsJSON(root, b1, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// change checksum triggers update
	b2 := &bookmark.Bookmark{
		URL:      "https://example.com/update",
		Title:    "New Title",
		Checksum: "new456",
	}
	updated, err := SaveAsJSON(root, b2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected update when checksum differs")
	}

	// verify file updated
	domain, _ := b2.Domain()
	hash := b2.HashURL()
	path := filepath.Join(root, domain, hash+".json")
	bj := bookmark.NewJSON()
	err = files.JSONRead(path, bj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bj.Checksum != "new456" {
		t.Errorf("expected checksum 'new456', got %s", bj.Checksum)
	}
}

func TestSaveAsJSON_ForceOverwrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b := &bookmark.Bookmark{
		URL:      "https://example.com/force",
		Title:    "Old",
		Checksum: "old123",
	}
	if _, err := SaveAsJSON(root, b, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	b.Title = "Forced"
	b.Checksum = "forced789"

	updated, err := SaveAsJSON(root, b, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected updated=true for force=true")
	}
}

func TestSaveAsJSON_InvalidDomain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := &bookmark.Bookmark{
		URL:      "://bad-url",
		Title:    "Invalid",
		Checksum: "inv123",
	}

	_, err := SaveAsJSON(root, b, false)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestSaveAsJSON_MultipleSameDomain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b1 := &bookmark.Bookmark{
		URL:      "https://example.com/page1",
		Title:    "Page 1",
		Checksum: "a",
	}
	b2 := &bookmark.Bookmark{
		URL:      "https://example.com/page2",
		Title:    "Page 2",
		Checksum: "b",
	}

	if _, err := SaveAsJSON(root, b1, false); err != nil {
		t.Fatalf("failed: %v", err)
	}
	if _, err := SaveAsJSON(root, b2, false); err != nil {
		t.Fatalf("failed: %v", err)
	}

	domain, _ := b1.Domain()
	entries, err := os.ReadDir(filepath.Join(root, domain))
	if err != nil {
		t.Fatalf("readDir failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 files for same domain, got %d", len(entries))
	}
}

// TestSaveAsJSONConcurrent tests concurrent writes to different domains.
func TestSaveAsJSONConcurrent(t *testing.T) {
	t.Parallel()
	rootPath := t.TempDir()

	bookmarks := []*bookmark.Bookmark{
		{URL: "https://example1.com/page", Title: "Page 1", Checksum: "check1"},
		{URL: "https://example2.com/page", Title: "Page 2", Checksum: "check2"},
		{URL: "https://example3.com/page", Title: "Page 3", Checksum: "check3"},
		{URL: "https://example4.com/page", Title: "Page 4", Checksum: "check4"},
	}

	errChan := make(chan error, len(bookmarks))

	for _, b := range bookmarks {
		go func(bookmark *bookmark.Bookmark) {
			_, err := SaveAsJSON(rootPath, bookmark, false)
			errChan <- err
		}(b)
	}

	for range bookmarks {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent SaveAsJSON failed: %v", err)
		}
	}
}

// TestSaveAsJSONDirectoryCreation tests that nested directories are created properly.
func TestSaveAsJSONDirectoryCreation(t *testing.T) {
	t.Parallel()
	rootPath := t.TempDir()

	b := &bookmark.Bookmark{
		URL:      "https://deeply.nested.example.com/path",
		Title:    "Deep Domain",
		Checksum: "deep123",
	}

	updated, err := SaveAsJSON(rootPath, b, false)
	if err != nil {
		t.Fatalf("SaveAsJSON() error = %v", err)
	}

	if !updated {
		t.Error("expected file to be created")
	}

	domain, _ := b.Domain()
	domainPath := filepath.Join(rootPath, domain)

	info, err := os.Stat(domainPath)
	if err != nil {
		t.Fatalf("domain directory not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected domain path to be a directory")
	}
}

// BenchmarkSaveAsJSON benchmarks the SaveAsJSON function.
func BenchmarkSaveAsJSON(b *testing.B) {
	rootPath := b.TempDir()

	bmBase := &bookmark.Bookmark{
		URL:      "https://example.com/benchmark",
		Title:    "Benchmark Page",
		Checksum: "bench123",
	}

	for i := 0; b.Loop(); i++ {
		// Create unique bookmark for each iteration
		bm := &bookmark.Bookmark{
			URL:      bmBase.URL + "/" + strconv.Itoa(i),
			Title:    bmBase.Title,
			Checksum: bmBase.Checksum,
		}
		_, err := SaveAsJSON(rootPath, bm, false)
		if err != nil {
			b.Fatalf("SaveAsJSON failed: %v", err)
		}
	}
}
