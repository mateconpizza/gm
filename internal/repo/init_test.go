//nolint:paralleltest //test
package repo

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/haaag/gm/internal/slice"
)

// setupTestDB sets up a test database.
func setupTestDB(t *testing.T) *SQLiteRepository {
	t.Helper()
	c, _ := NewSQLiteCfg("")
	db, err := openDatabase("file:testdb?mode=memory&cache=shared")
	assert.NoError(t, err, "failed to open database")
	r := newSQLiteRepository(db, c)
	_ = r.Init()

	return r
}

// teardownthewall closes the database connection.
func teardownthewall(db *sqlx.DB) {
	if err := db.Close(); err != nil {
		slog.Error("closing database", "error", err)
	}
}

func testSingleBookmark() *Row {
	return &Row{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01T12:00:00Z",
		LastVisit: "2023-01-01T12:00:00Z",
		Favorite:  true,
	}
}

func testSliceBookmarks() *Slice {
	s := slice.New[Row]()
	for i := 0; i < 10; i++ {
		b := testSingleBookmark()
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)
		s.Push(b)
	}

	return s
}

func testPopulatedDB(t *testing.T) *SQLiteRepository {
	t.Helper()
	r := setupTestDB(t)
	bs := testSliceBookmarks()
	ctx := context.Background()
	err := r.InsertMany(ctx, bs)
	assert.NoError(t, err)

	return r
}

func TestInit(t *testing.T) {
	c, _ := NewSQLiteCfg("")
	db, err := openDatabase("file:testdb?mode=memory&cache=shared")
	assert.NoError(t, err, "failed to open database")
	r := newSQLiteRepository(db, c)
	assert.NoError(t, r.Init(), "failed to initialize repository")
	defer teardownthewall(r.DB)
	for _, s := range tablesAndSchema() {
		tExists, err := r.tableExists(s.name)
		assert.NoError(t, err)
		assert.True(t, tExists, "main table does not exist")
	}
}

func TestDropTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	tDrop := schemaMain.name
	err := r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableDrop(tx, tDrop)
	})
	assert.NoError(t, err, "failed to drop table %s", tDrop)
	_, err = r.DB.Exec(fmt.Sprintf("SELECT * FROM %s", tDrop))
	assert.Error(t, err, "main table still exists after calling HandleDropDB")
	exists, err := r.tableExists(tDrop)
	assert.NoError(t, err)
	assert.False(t, exists, "tableExists returned true for a non-existent table")
}

func TestTableCreate(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	var newTable Table = "new_table"
	assert.NoError(t, r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableCreate(tx, newTable, "CREATE TABLE new_table (id INTEGER PRIMARY KEY)")
	}), "failed to create table %s", newTable)
	exists, err := r.tableExists(newTable)
	assert.NoError(t, err)
	assert.True(t, exists, "tableExists returned false for a non-existent table")
}

func TestTableExists(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	var tt Table = "test_table"
	assert.NoError(t, r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableCreate(tx, tt, "CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
	}), "failed to create table %s", tt)
	exists, err := r.tableExists(tt)
	assert.NoError(t, err)
	assert.True(t, exists, "tableExists returned false for an existing table")
	exists, err = r.tableExists("non_existent_table")
	assert.NoError(t, err)
	assert.False(t, exists, "tableExists returned false for a non-existent table")
}

func TestRenameTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	srcTable := schemaMain.name
	destTable := "new_" + srcTable
	err := r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableRename(tx, srcTable, destTable)
	})
	assert.NoError(t, err, "failed to rename table %s to %s", srcTable, destTable)
	srcExists, err := r.tableExists(srcTable)
	assert.False(t, srcExists, "table %s does not exist", srcTable)
	assert.NoError(t, err, "failed to check if table %s exists", srcTable)
	destExists, err := r.tableExists(destTable)
	assert.NoError(t, err, "failed to check if table %s exists", destTable)
	assert.True(t, destExists, "table %s does not exist", destTable)
}
