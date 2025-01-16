package repo

import (
	"database/sql"
	"fmt"
	"log"
	"testing"

	"github.com/haaag/gm/internal/slice"
)

func setupTestDB(t *testing.T) *SQLiteRepository {
	t.Helper()

	c := NewSQLiteCfg("")
	db, err := MustOpenDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.Init(); err != nil {
		t.Fatal(err)
	}

	return r
}

// teardownthewall closes the database connection.
func teardownthewall(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Printf("Error closing rows: %v", err)
	}
}

func getBookmark() *Row {
	return &Row{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
	}
}

func getValidBookmarks() *Slice {
	s := slice.New[Row]()

	for i := 0; i < 10; i++ {
		b := getBookmark()
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)
		s.Append(b)
	}

	return s
}

func TestDropTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tDrop := r.Cfg.Tables.Main

	err := r.TableDrop(tDrop)
	if err != nil {
		t.Errorf("Error dropping table: %v", err)
	}

	_, err = r.DB.Exec(fmt.Sprintf("SELECT * FROM %s", tDrop))
	if err == nil {
		t.Errorf("DBMainTable still exists after calling HandleDropDB: %v", err)
	}

	_, err = r.DB.Exec(fmt.Sprintf("SELECT * FROM %s", tDrop))
	if err == nil {
		t.Errorf("DBDeletedTable still exists after calling HandleDropDB")
	}
}

func TestTableExists(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	var tt Table = "test_table"
	err := r.TableCreate(tt, tableMainSchema)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := r.tableExists(tt)
	if err != nil {
		t.Fatal(err)
	}

	if !exists {
		t.Error("TableExists returned false for an existing table")
	}

	exists, err = r.tableExists("non_existent_table")
	if err != nil {
		t.Fatal(err)
	}

	if exists {
		t.Error("TableExists returned true for a non-existent table")
	}
}

func TestTableCreate(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	var newTable Table = "new_table"
	if err := r.TableCreate(newTable, tableMainSchema); err != nil {
		t.Errorf("Error creating table: %v", err)
	}

	exists, err := r.tableExists(newTable)
	if !exists {
		t.Errorf("Table %s does not exist", newTable)
	}
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}
}

func TestRenameTable(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	srcTable := r.Cfg.Tables.Main
	destTable := "new_" + srcTable

	err := r.tableRename(srcTable, destTable)
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
