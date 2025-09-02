//nolint:paralleltest,wsl //test
package db

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

// setupTestDB sets up a test database.
func setupTestDB(t *testing.T) *SQLite {
	t.Helper()
	c, _ := NewSQLiteCfg("")
	db, err := OpenDatabase(fmt.Sprintf("file:testdb_%d?mode=memory", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	r := newSQLiteRepository(db, c)
	_ = r.Init(t.Context())

	return r
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
	db, err := OpenDatabase("file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.Init(t.Context()); err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}
	defer teardownthewall(r.DB)

	for _, s := range tablesAndSchemas {
		tExists, err := tableExists(r, s.Name)
		if err != nil {
			t.Errorf("failed to check if table %s exists: %v", s.Name, err)
			continue
		}
		if !tExists {
			t.Errorf("expected table %s to exist, but it does not", s.Name)
		}
	}
}

func TestDropTable(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tDrop := schemaMain.Name
	err := r.WithTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.tableDrop(t.Context(), tx, tDrop)
	})
	if err != nil {
		t.Fatalf("failed to drop table %s: %v", tDrop, err)
	}

	_, err = r.DB.ExecContext(t.Context(), fmt.Sprintf("SELECT * FROM %s", tDrop))
	if err == nil {
		t.Errorf("main table %s still exists after dropping", tDrop)
	}

	exists, err := tableExists(r, tDrop)
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	if exists {
		t.Errorf("expected table %s to not exist, but it still does", tDrop)
	}
}

func TestTableCreate(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	var newTable Table = "new_table"
	err := r.WithTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.tableCreate(t.Context(), tx, newTable, "CREATE TABLE new_table (id INTEGER PRIMARY KEY)")
	})
	if err != nil {
		t.Fatalf("failed to create table %s: %v", newTable, err)
	}

	exists, err := tableExists(r, newTable)
	if err != nil {
		t.Fatalf("failed to check if table exists: %v", err)
	}
	if !exists {
		t.Errorf("expected table %s to exist, but it does not", newTable)
	}
}

func TestTableExists(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	var tt Table = "test_table"
	err := r.WithTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.tableCreate(t.Context(), tx, tt, "CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
	})
	if err != nil {
		t.Fatalf("failed to create table %s: %v", tt, err)
	}

	exists, err := tableExists(r, tt)
	if err != nil {
		t.Fatalf("failed to check table %s existence: %v", tt, err)
	}
	if !exists {
		t.Errorf("expected table %s to exist, but it does not", tt)
	}

	exists, err = tableExists(r, "non_existent_table")
	if err != nil {
		t.Fatalf("failed to check non-existent table: %v", err)
	}
	if exists {
		t.Errorf("expected non-existent table to not exist, but it does")
	}
}

func TestRenameTable(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	srcTable := schemaMain.Name
	destTable := "new_" + srcTable

	err := r.WithTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.tableRename(t.Context(), tx, srcTable, destTable)
	})
	if err != nil {
		t.Fatalf("failed to rename table %s to %s: %v", srcTable, destTable, err)
	}

	srcExists, err := tableExists(r, srcTable)
	if err != nil {
		t.Fatalf("failed to check if source table exists: %v", err)
	}
	if srcExists {
		t.Errorf("expected source table %s to not exist, but it does", srcTable)
	}

	destExists, err := tableExists(r, destTable)
	if err != nil {
		t.Fatalf("failed to check if destination table exists: %v", err)
	}
	if !destExists {
		t.Errorf("expected destination table %s to exist, but it does not", destTable)
	}
}
