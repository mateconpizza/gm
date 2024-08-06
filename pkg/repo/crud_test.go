package repo

import (
	"database/sql"
	"fmt"
	"log"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/pkg/app"
	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/slice"
)

const tempTableName = "test_table"

var DBCfg = NewSQLiteCfg()

func setupTestDB(t *testing.T) (*sql.DB, *SQLiteRepository) {
	t.Helper()
	// Set up an in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// test_table
	sqlQuery := fmt.Sprintf(
		"CREATE TABLE %s (id INTEGER PRIMARY KEY, url TEXT, title TEXT, tags TEXT, desc TEXT, created_at TEXT)",
		tempTableName,
	)
	_, err = db.Exec(sqlQuery)
	if err != nil {
		t.Fatal(err)
	}

	DBCfg.SetName(app.DefaultDBName)
	r := newSQLiteRepository(db, DBCfg)

	return db, r
}

func teardownTestDB(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Printf("Error closing rows: %v", err)
	}
}

func getValidBookmark() Row {
	return Row{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,testme,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
	}
}

func getValidBookmarks() *Slice {
	s := slice.New[Row]()

	for i := 0; i < 10; i++ {
		b := getValidBookmark()
		b.Title = fmt.Sprintf("Title %d", i)
		s.Add(&b)
	}

	return s
}

func TestDropTable(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	err := r.tableDrop(tempTableName)
	if err != nil {
		t.Errorf("Error dropping table: %v", err)
	}

	//nolint:perfsprint //gosec conflict
	_, err = db.Exec(fmt.Sprintf("SELECT * FROM %s", DBCfg.GetTableMain()))
	if err == nil {
		t.Errorf("DBMainTable still exists after calling HandleDropDB")
	}

	//nolint:perfsprint //gosec conflict
	_, err = db.Exec(fmt.Sprintf("SELECT * FROM %s", DBCfg.GetTableDeleted()))
	if err == nil {
		t.Errorf("DBDeletedTable still exists after calling HandleDropDB")
	}
}

func TestTableCreate(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	tableName := "new_table"

	if err := r.TableCreate(tableName, tableMainSchema); err != nil {
		t.Errorf("Error creating table: %v", err)
	}

	exists, err := r.tableExists(tableName)
	if !exists {
		t.Errorf("Table %s does not exist", tableName)
	}
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}
}

func TestTableExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	exists, err := r.tableExists(tempTableName)
	if err != nil {
		t.Fatal(err)
	}

	if !exists {
		t.Error("TableExists returned false for an existing table")
	}
}

func TestTableDoesNotExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	exists, err := r.tableExists("non_existent_table")
	if err != nil {
		t.Fatal(err)
	}

	if exists {
		t.Error("TableExists returned true for a non-existent table")
	}
}

func TestCreateRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	b := &Row{
		URL:       "https://example.com",
		Title:     "Title",
		Tags:      "test",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
	}

	created, err := r.Insert(tempTableName, b)
	if err != nil {
		t.Fatal(err)
	}

	if created.ID == 0 {
		t.Error("InsertRecord did not return a valid ID")
	}

	// Insert a duplicate record
	duplicate := &Row{
		URL: "https://example.com",
	}

	_, err = r.Insert(tempTableName, duplicate)
	if err == nil {
		t.Error("InsertRecord did not return an error for a duplicate record")
	}

	// Insert an invalid record
	invalidBookmark := &Row{}

	_, err = r.Insert(tempTableName, invalidBookmark)
	if err == nil {
		t.Error("InsertRecord did not return an error for an invalid record")
	}
}

func TestDeleteRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	b := getValidBookmark()

	_, err := r.Insert(tempTableName, &b)
	if err != nil {
		t.Fatal(err)
	}

	// Test the deletion of a valid record
	err = r.delete(tempTableName, &b)
	if err != nil {
		t.Errorf("DeleteRecord returned an error: %v", err)
	}

	// Test the deletion of a non-existent record
	err = r.delete(tempTableName, &b)
	if err == nil {
		t.Error("DeleteRecord did not return an error for a non-existent record")
	}
}

func TestDeleteRecordBulk(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert valid records
	bs := getValidBookmarks()
	if err := r.insertBulk(tempTableName, bs); err != nil {
		t.Fatal(err)
	}

	// Verify that the record was inserted successfully
	newRows := slice.New[Row]()
	if err := r.GetAll(tempTableName, newRows); err != nil {
		t.Fatal(err)
	}

	if newRows.Len() != bs.Len() {
		t.Errorf("Unexpected number of bookmarks retrieved: got %d, expected %d", newRows.Len(), 2)
	}

	// Delete the record and verify that it was deleted successfully
	ids := slice.New[int]()
	newRows.ForEach(func(r Row) {
		ids.Add(&r.ID)
	})

	if ids.Len() != newRows.Len() {
		t.Errorf(
			"Unexpected number of IDs retrieved: got %d, expected %d",
			ids.Len(),
			newRows.Len(),
		)
	}

	// Test the deletion of a valid record
	err := r.DeleteBulk(tempTableName, ids)
	if err != nil {
		t.Errorf("DeleteRecordBulk returned an error: %v", err)
	}

	emptyRows := slice.New[Row]()
	_ = r.GetAll(tempTableName, emptyRows)

	if emptyRows.Len() != 0 {
		t.Errorf(
			"Unexpected number of bookmarks retrieved: got %d, expected %d",
			emptyRows.Len(),
			0,
		)
	}
}

func TestIsRecordExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	b := &Row{
		URL: "https://example.com",
	}

	query := fmt.Sprintf("INSERT INTO %s (url) VALUES (?)", tempTableName)

	_, err := db.Exec(query, b.URL)
	if err != nil {
		t.Fatal(err)
	}

	exists := r.HasRecord(tempTableName, "url", b.URL)
	if !exists {
		t.Errorf("isRecordExists returned false for an existing record")
	}

	nonExistentBookmark := &Row{
		URL: "https://non_existent.com",
	}

	exists = r.HasRecord(tempTableName, "url", nonExistentBookmark.URL)
	if exists {
		t.Errorf("isRecordExists returned true for a non-existent record")
	}
}

func TestUpdateRecordSuccess(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	validB := getValidBookmark()

	q := fmt.Sprintf("INSERT INTO %s (url) VALUES (?)", tempTableName)

	result, err := db.Exec(q, validB.URL)
	if err != nil {
		t.Errorf("Error inserting bookmark: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Errorf("Error getting last insert ID: %v", err)
	}

	validB.ID = int(id)

	_, err = r.Update(tempTableName, &validB)
	if err != nil {
		t.Error(err)
	}

	q = fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tempTableName)
	row := db.QueryRow(q, validB.ID)

	var b Row

	err = row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt)
	if err != nil {
		t.Errorf("Error scanning row: %v", err)
	}

	if b.ID != validB.ID {
		t.Errorf("Error updating bookmark: %v", ErrRecordUpdate)
	}
}

func TestUpdateRecordError(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	_, err := r.Update(tempTableName, &Row{})
	if err == nil {
		t.Error("UpdateRecord did not return an error for an invalid record")
	}
}

func TestGetRecordByID(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	b := getValidBookmark()

	inserted, err := r.Insert(tempTableName, &b)
	if err != nil {
		t.Fatal(err)
	}

	record, err := r.GetByID(tempTableName, inserted.ID)
	if err != nil {
		t.Errorf("Error getting bookmark by ID: %v", err)
	}

	if record.ID != inserted.ID || record.URL != inserted.URL {
		t.Errorf("Unexpected bookmark retrieved: got %+v, expected %+v", record, inserted)
	}
}

func TestGetRecordsByQuery(t *testing.T) {
	expectedRecords := 2
	db, r := setupTestDB(t)

	defer teardownTestDB(db)

	b := getValidBookmark()
	_, _ = r.Insert(tempTableName, &b)
	b.URL = "https://www.example2.com"
	_, _ = r.Insert(tempTableName, &b)
	b.URL = "https://www.another.com"

	bs := slice.New[Row]()
	if err := r.GetByQuery(tempTableName, "example", bs); err != nil {
		t.Errorf("Error getting bookmarks by query: %v", err)
	}

	n := bs.Len()

	if n != expectedRecords {
		t.Errorf("Unexpected number of bookmarks retrieved: got %d, expected %d", n, 2)
	}

	var count int
	//nolint:perfsprint //gosec conflict
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tempTableName)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != n {
		t.Errorf("Expected %d records, got %d", n, count)
	}
}

func TestInsertRecordsBulk(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	bookmarks := []Row{
		{
			URL:       "url1",
			Title:     "title1",
			Tags:      "tag1",
			Desc:      "desc1",
			CreatedAt: "2023-01-01 12:00:00",
		},
		{
			URL:       "url2",
			Title:     "title2",
			Tags:      "tag2",
			Desc:      "desc2",
			CreatedAt: "2023-01-01 12:00:00",
		},
		{
			URL:       "url3",
			Title:     "title2",
			Tags:      "tag3",
			Desc:      "desc2",
			CreatedAt: "2023-01-01 12:00:00",
		},
	}

	bs := slice.New[Row]()
	bs.Set(&bookmarks)
	err := r.insertBulk(tempTableName, bs)
	if err != nil {
		t.Fatal(err)
	}

	n := len(bookmarks)
	var count int
	//nolint:perfsprint //gosec conflict
	err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tempTableName)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != n {
		t.Errorf("Expected %d records, got %d", n, count)
	}
}

func TestRenameTable(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	err := r.tableRename(tempTableName, DBCfg.GetTableDeleted())
	if err != nil {
		t.Errorf("Error renaming table: %v", err)
	}

	exists, err := r.tableExists(DBCfg.GetTableDeleted())
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}

	if !exists {
		t.Errorf("Table %s does not exist", DBCfg.GetTableDeleted())
	}
}

func TestRecordIsValid(t *testing.T) {
	validBookmark := Row{
		Title: "Example",
		URL:   "https://www.example.com",
		Tags:  "tag1,tag2",
	}

	if err := bookmark.Validate(&validBookmark); err != nil {
		t.Errorf("TestBookmarkIsValid: expected valid bookmark to be valid")
	}

	invalidBookmark := Row{
		Title: "",
		URL:   "",
	}

	if err := bookmark.Validate(&invalidBookmark); err == nil {
		t.Errorf("TestBookmarkIsValid: expected invalid bookmark to be invalid")
	}
}
