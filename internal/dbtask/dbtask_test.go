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
