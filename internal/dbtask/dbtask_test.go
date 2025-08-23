package dbtask

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// setupTestDB sets up a test database.
func setupTestDB(t *testing.T) *db.SQLite {
	t.Helper()
	c, _ := db.NewSQLiteCfg("")
	repo, err := db.OpenDatabase(
		fmt.Sprintf("file:testdb_%d?mode=memory&cache=shared", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	r := &db.SQLite{
		DB:  repo,
		Cfg: c,
	}
	_ = r.Init(t.Context())

	return r
}

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
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)
		bs = append(bs, b)
	}

	return bs
}

func testPopulatedDB(t *testing.T, n int) *db.SQLite {
	t.Helper()
	r := setupTestDB(t)
	bs := testSliceBookmarks(n)
	ctx := t.Context()
	err := r.InsertMany(ctx, bs)
	if err != nil {
		t.Fatalf("failed to insert bookmarks: %v", err)
	}

	return r
}

// teardownthewall closes the database connection.
func teardownthewall(repo *sqlx.DB) {
	if err := repo.Close(); err != nil {
		slog.Error("closing database", "error", err)
	}
}

func TestRecordIDs(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	// get initial records
	bs, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("AllPtr failed: %v", err)
	}
	if len(bs) != want {
		t.Fatalf("expected %d records, got %d", want, len(bs))
	}

	// delete records at indices 1, 2, 5 (ids 2, 3, 6)
	toDelete := []*bookmark.Bookmark{bs[1], bs[2], bs[5]}
	if err := r.DeleteMany(t.Context(), toDelete); err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	// verify deletion - should have 7 records left
	remaining, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("All after deletion failed: %v", err)
	}
	if len(remaining) != 7 {
		t.Fatalf("expected 7 records after deletion, got %d", len(remaining))
	}

	// extract and verify current ids
	currentIDs := extractIDs(remaining)
	expectedAfterDelete := []int{1, 4, 5, 7, 8, 9, 10}
	if !equalIntSlices(currentIDs, expectedAfterDelete) {
		t.Fatalf("after deletion, expected IDs %v, got %v", expectedAfterDelete, currentIDs)
	}

	// reorder ids
	if err := DeleteAndReorder(t.Context(), r); err != nil {
		t.Fatalf("ReorderIDs failed: %v", err)
	}

	// verify reordering - ids should be 1-7 consecutively
	reordered, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("AllPtr after reordering failed: %v", err)
	}
	if len(reordered) != 7 {
		t.Fatalf("expected 7 records after reordering, got %d", len(reordered))
	}

	reorderedIDs := extractIDs(reordered)
	expectedReordered := []int{1, 2, 3, 4, 5, 6, 7}
	if !equalIntSlices(reorderedIDs, expectedReordered) {
		t.Fatalf("after reordering, expected IDs %v, got %v", expectedReordered, reorderedIDs)
	}
}

func extractIDs(bookmarks []*bookmark.Bookmark) []int {
	ids := make([]int, len(bookmarks))
	for i, b := range bookmarks {
		ids[i] = b.ID
	}
	return ids
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
