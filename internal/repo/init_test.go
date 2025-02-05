package repo

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/haaag/gm/internal/slice"
)

// setupTestDB sets up a test database.
func setupTestDB(t *testing.T) *SQLiteRepository {
	t.Helper()
	c := NewSQLiteCfg("")
	db, err := openDatabase("file:testdb?mode=memory&cache=shared")
	assert.NoError(t, err, "failed to open database")
	r := newSQLiteRepository(db, c)
	for name, schema := range r.tablesAndSchema() {
		s := fmt.Sprintf(schema, name)
		if _, err := db.Exec(s); err != nil {
			assert.NoError(t, err, "failed to create table %s", name)
		}
	}

	return r
}

// teardownthewall closes the database connection.
func teardownthewall(db *sqlx.DB) {
	if err := db.Close(); err != nil {
		log.Printf("Error closing rows: %v", err)
	}
}

func testSingleBookmark() *Row {
	return &Row{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
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

func TestInit(t *testing.T) {
	c := NewSQLiteCfg("")
	db, err := openDatabase("file:testdb?mode=memory&cache=shared")
	assert.NoError(t, err, "failed to open database")
	r := newSQLiteRepository(db, c)
	assert.NoError(t, r.Init(), "failed to initialize repository")
	defer teardownthewall(r.DB)
	tables := []Table{
		r.Cfg.Tables.Main,
		r.Cfg.Tables.Deleted,
		r.Cfg.Tables.Tags,
		r.Cfg.Tables.RecordsTags,
		r.Cfg.Tables.RecordsTagsDeleted,
	}
	for _, table := range tables {
		tExists, err := r.tableExists(table)
		assert.NoError(t, err)
		assert.True(t, tExists, "main table does not exist")
	}
}

func TestDropTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	tDrop := r.Cfg.Tables.Main
	err := r.execTx(context.Background(), func(tx *sqlx.Tx) error {
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
	assert.NoError(t, r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableCreate(tx, newTable, tableMainSchema)
	}), "failed to create table %s", newTable)
	exists, err := r.tableExists(newTable)
	assert.NoError(t, err)
	assert.True(t, exists, "tableExists returned false for a non-existent table")
}

func TestTableExists(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	var tt Table = "test_table"
	assert.NoError(t, r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.tableCreate(tx, tt, tableMainSchema)
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
	srcTable := r.Cfg.Tables.Main
	destTable := "new_" + srcTable
	err := r.execTx(context.Background(), func(tx *sqlx.Tx) error {
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

func TestSize(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	b := testSingleBookmark()
	err := r.Insert(b)
	assert.NoError(t, err, "failed to insert test data")
	dbSize, err := r.size()
	assert.NoError(t, err, "failed to get DB size")
	assert.GreaterOrEqual(
		t,
		dbSize,
		int64(1000),
		"expected database size to be less than 1000 bytes",
	)
}
