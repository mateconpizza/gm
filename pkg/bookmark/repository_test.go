package bookmark

import (
	"database/sql"
	"fmt"
	"log"
	"testing"

	"gomarks/pkg/config"

	_ "github.com/mattn/go-sqlite3"
)

var tempTableName = "test_table"

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

	r := NewSQLiteRepository(db)

	return db, r
}

func teardownTestDB(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Printf("Error closing rows: %v", err)
	}
}

func getValidBookmark() Bookmark {
	return Bookmark{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,testme,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
	}
}

func TestDropTable(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	err := r.dropTable(tempTableName)
	if err != nil {
		t.Errorf("Error dropping table: %v", err)
	}

	_, err = db.Exec(fmt.Sprintf("SELECT * FROM %s", config.DB.Table.Main))
	if err == nil {
		t.Errorf("DBMainTable still exists after calling HandleDropDB")
	}

	_, err = db.Exec(fmt.Sprintf("SELECT * FROM %s", config.DB.Table.Deleted))
	if err == nil {
		t.Errorf("DBDeletedTable still exists after calling HandleDropDB")
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

func TestInsertRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	b := &Bookmark{
		URL:       "https://example.com",
		Title:     "Title",
		Tags:      "test",
		Desc:      "Description",
		CreatedAt: "2023-01-01 12:00:00",
	}

	inserted, err := r.Create(tempTableName, b)
	if err != nil {
		t.Fatal(err)
	}

	if inserted.ID == 0 {
		t.Error("InsertRecord did not return a valid ID")
	}

	// Insert a duplicate record
	duplicate := &Bookmark{
		URL: "https://example.com",
	}

	_, err = r.Create(tempTableName, duplicate)
	if err == nil {
		t.Error("InsertRecord did not return an error for a duplicate record")
	}

	// Insert an invalid record
	invalidBookmark := &Bookmark{}

	_, err = r.Create(tempTableName, invalidBookmark)
	if err == nil {
		t.Error("InsertRecord did not return an error for an invalid record")
	}
}

func TestDeleteRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	b := getValidBookmark()

	_, err := r.Create(tempTableName, &b)
	if err != nil {
		t.Fatal(err)
	}

	// Test the deletion of a valid record
	err = r.Delete(tempTableName, &b)
	if err != nil {
		t.Errorf("DeleteRecord returned an error: %v", err)
	}

	// Test the deletion of a non-existent record
	err = r.Delete(tempTableName, &b)
	if err == nil {
		t.Error("DeleteRecord did not return an error for a non-existent record")
	}
}

func TestIsRecordExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	b := &Bookmark{
		URL: "https://example.com",
	}

	query := fmt.Sprintf("INSERT INTO %s (url) VALUES (?)", tempTableName)

	_, err := db.Exec(query, b.URL)
	if err != nil {
		t.Fatal(err)
	}

	exists := r.RecordExists(tempTableName, "url", b.URL)
	if !exists {
		t.Errorf("isRecordExists returned false for an existing record")
	}

	nonExistentBookmark := &Bookmark{
		URL: "https://non_existent.com",
	}

	exists = r.RecordExists(tempTableName, "url", nonExistentBookmark.URL)
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

	var b Bookmark

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

	_, err := r.Update(tempTableName, &Bookmark{})
	if err == nil {
		t.Error("UpdateRecord did not return an error for an invalid record")
	}
}

func TestGetRecordByID(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	b := getValidBookmark()

	inserted, err := r.Create(tempTableName, &b)
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
	_, _ = r.Create(tempTableName, &b)
	b.URL = "https://www.example2.com"
	_, _ = r.Create(tempTableName, &b)
	b.URL = "https://www.another.com"

	records, err := r.GetByQuery(tempTableName, "example")
	if err != nil {
		t.Errorf("Error getting bookmarks by query: %v", err)
	}

	if len(*records) != expectedRecords {
		t.Errorf("Unexpected number of bookmarks retrieved: got %d, expected %d", len(*records), 2)
	}

	var count int
	err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tempTableName)).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != len(*records) {
		t.Errorf("Expected %d records, got %d", len(*records), count)
	}
}

func TestInsertRecordsBulk(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	bookmarks := []Bookmark{
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

	err := r.CreateBulk(tempTableName, &bookmarks)
	if err != nil {
		t.Fatal(err)
	}

	var count int

	err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tempTableName)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != len(bookmarks) {
		t.Errorf("Expected %d records, got %d", len(bookmarks), count)
	}
}

func TestRenameTable(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	err := r.tableRename(tempTableName, config.DB.Table.Deleted)
	if err != nil {
		t.Errorf("Error renaming table: %v", err)
	}

	exists, err := r.tableExists(config.DB.Table.Deleted)
	if err != nil {
		t.Errorf("Error checking if table exists: %v", err)
	}

	if !exists {
		t.Errorf("Table %s does not exist", config.DB.Table.Deleted)
	}
}

func TestBookmarkIsValid(t *testing.T) {
	validBookmark := Bookmark{
		Title: "Example",
		URL:   "https://www.example.com",
		Tags:  "tag1,tag2",
	}

	if err := Validate(&validBookmark); err != nil {
		t.Errorf("TestBookmarkIsValid: expected valid bookmark to be valid")
	}

	invalidBookmark := Bookmark{
		Title: "",
		URL:   "",
	}

	if err := Validate(&invalidBookmark); err == nil {
		t.Errorf("TestBookmarkIsValid: expected invalid bookmark to be invalid")
	}
}
