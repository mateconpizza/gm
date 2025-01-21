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

func setupTestDB(t *testing.T) *SQLiteRepository {
	t.Helper()

	c := NewSQLiteCfg("")
	db, err := MustOpenDatabase(":memory:")
	assert.NoError(t, err, "Failed to open database")

	r := newSQLiteRepository(db, c)
	assert.NoError(t, r.Init(), "Failed to initialize repository")

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
		s.Append(b)
	}

	return s
}

func TestInit(t *testing.T) {
	c := NewSQLiteCfg("")
	db, err := MustOpenDatabase(":memory:")
	assert.NoError(t, err, "failed to open database")

	r := newSQLiteRepository(db, c)
	err = r.Init()
	assert.NoError(t, err, "failed to initialize repository")
	defer teardownthewall(r.DB)
	tables := []Table{
		r.Cfg.Tables.Main,
		r.Cfg.Tables.Deleted,
		r.Cfg.Tables.Tags,
		r.Cfg.Tables.RecordsTags,
		r.Cfg.Tables.RecordsTagsDeleted,
	}

	for _, table := range tables {
		_, err := r.DB.Exec(fmt.Sprintf("SELECT * FROM %s", table))
		assert.NoError(t, err, "failed to query table %s", table)
	}
}

func TestDropTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tDrop := r.Cfg.Tables.Main
	_ = r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		err := r.tableDrop(tx, tDrop)
		assert.NoError(t, err, "failed to drop table %s", tDrop)

		return nil
	})

	_, err := r.DB.Exec(fmt.Sprintf("SELECT * FROM %s", tDrop))
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
		err := r.tableRename(tx, srcTable, destTable)
		if err != nil {
			t.Errorf("Error renaming table: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Errorf("Error renaming table: %v", err)
	}

	srcExists, err := r.tableExists(srcTable)
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}

	if srcExists {
		t.Errorf("Table '%s' still exists", srcTable)
	}

	destExists, err := r.tableExists(destTable)
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}

	if !destExists {
		t.Errorf("Table %s does not exist", r.Cfg.Tables.Deleted)
	}
}

func TestSize(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	_, err := r.DB.Exec(
		`INSERT INTO bookmarks (id, url, title, desc) VALUES (1, 'http://example.com', 'Example', 'Description')`,
	)
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
