package db

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

// setupTestDB sets up an isolated, migrated in-memory database for a single test.
func setupTestDB(t *testing.T) *SQLite {
	t.Helper()

	c, err := NewSQLiteCfg("")
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Using t.Name() ensures unique DBs per test/sub-test, while cache=shared
	// keeps the pool unified.
	// Clean the name of characters that might upset a file-based DSN string.
	safeTestName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", safeTestName)

	db, err := OpenDatabase(t.Context(), dsn, c)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Close the DB automatically when the test finishes to wipe the in-memory
	// cache
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("warning: failed to close test db: %v", err)
		}
	})

	r := newSQLiteRepository(db, c)

	ms, err := LoadMigrations()
	if err != nil {
		t.Fatalf("failed to load migrations: %v", err)
	}

	if err := Migrate(t.Context(), r, ms); err != nil {
		t.Fatalf("migration failed during setup: %v", err)
	}

	return r
}

// setupTestDBNoMigration sets up an isolated, blank in-memory database.
func setupTestDBNoMigration(t *testing.T) *SQLite {
	t.Helper()

	c, err := NewSQLiteCfg("")
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	safeTestName := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s_nomig?mode=memory&cache=shared", safeTestName)

	db, err := OpenDatabase(t.Context(), dsn, c)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return newSQLiteRepository(db, c)
}

// teardownthewall closes the database connection.
func teardownthewall(db *sqlx.DB) {
	if err := db.Close(); err != nil {
		slog.Error("closing database", "error", err)
	}
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

func testPopulatedDB(t *testing.T, n int) *SQLite {
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

func TestInit(t *testing.T) {
	t.Parallel()
	c, _ := NewSQLiteCfg("")
	db, err := OpenDatabase(t.Context(), "file:testdb?mode=memory&cache=shared", c)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.Init(t.Context()); err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	for _, table := range tables {
		tExists, err := table.Exists(t.Context(), r)
		if err != nil {
			t.Errorf("failed to check if table %s exists: %v", table, err)
			continue
		}
		if !tExists {
			t.Errorf("expected table %s to exist, but it does not", table)
		}
	}
}
