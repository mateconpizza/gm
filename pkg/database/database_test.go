package database_test

import (
	"database/sql"
	"fmt"
	c "gomarks/pkg/constants"
	"gomarks/pkg/database"
	"testing"
)

func setupTestDB(t *testing.T) (*sql.DB, *database.SQLiteRepository) {
	// Set up an in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// test_table
	_, err = db.Exec(
		"CREATE TABLE test_table (id INTEGER PRIMARY KEY, url TEXT, title TEXT, tags TEXT, desc TEXT, created_at TEXT)",
	)
	if err != nil {
		t.Fatal(err)
	}

	r := database.NewSQLiteRepository(db)
	return db, r
}

func teardownTestDB(db *sql.DB) {
	db.Close()
}

func getValidBookmark() database.Bookmark {
	return database.Bookmark{
		URL:   "https://www.example.com",
		Title: database.NullString{NullString: sql.NullString{String: "Title", Valid: true}},
		Tags:  "test",
		Desc: database.NullString{
			NullString: sql.NullString{String: "Description", Valid: true},
		},
		Created_at: "2023-01-01 12:00:00",
	}
}

func TestHandleDropDB(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	r.HandleDropDB()

	_, err := db.Exec(fmt.Sprintf("SELECT * FROM %s", c.DBMainTable))
	if err == nil {
		t.Errorf("DBMainTable still exists after calling HandleDropDB")
	}

	_, err = db.Exec(fmt.Sprintf("SELECT * FROM %s", c.DBDeletedTable))
	if err == nil {
		t.Errorf("DBDeletedTable still exists after calling HandleDropDB")
	}
}

func TestTableExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	exists, err := r.TableExists("test_table")
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

	exists, err := r.TableExists("non_existent_table")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("TableExists returned true for a non-existent table")
	}
}

func TestInsertRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	bookmark := &database.Bookmark{
		URL:   "https://example.com",
		Title: database.NullString{NullString: sql.NullString{String: "Title", Valid: true}},
		Tags:  "test",
		Desc: database.NullString{
			NullString: sql.NullString{String: "Description", Valid: true},
		},
		Created_at: "2023-01-01 12:00:00",
	}
	inserted, err := r.InsertRecord(bookmark, "test_table")
	if err != nil {
		t.Fatal(err)
	}
	if inserted.ID == 0 {
		t.Error("InsertRecord did not return a valid ID")
	}

	// Insert a duplicate record
	duplicate := &database.Bookmark{
		URL: "https://example.com",
	}
	_, err = r.InsertRecord(duplicate, "test_table")
	if err == nil {
		t.Error("InsertRecord did not return an error for a duplicate record")
	}

	// Insert an invalid record
	invalidBookmark := &database.Bookmark{}
	_, err = r.InsertRecord(invalidBookmark, "test_table")
	if err == nil {
		t.Error("InsertRecord did not return an error for an invalid record")
	}
}

func TestDeleteRecord(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	// Insert a valid record
	bookmark := getValidBookmark()
	_, err := r.InsertRecord(&bookmark, "test_table")
	if err != nil {
		t.Fatal(err)
	}

	// Prueba la eliminación exitosa
	err = r.DeleteRecord(&bookmark, "test_table")
	if err != nil {
		t.Errorf("DeleteRecord returned an error: %v", err)
	}

	// Prueba la eliminación de un registro que no existe
	err = r.DeleteRecord(&bookmark, "test_table")
	if err == nil {
		t.Error("DeleteRecord did not return an error for a non-existent record")
	}
}

func TestIsRecordExists(t *testing.T) {
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	bookmark := &database.Bookmark{
		URL: "https://example.com",
	}

	_, err := db.Exec("INSERT INTO test_table (url) VALUES (?)", bookmark.URL)
	if err != nil {
		t.Fatal(err)
	}

	exists := r.RecordExists(bookmark, "test_table")
	if !exists {
		t.Errorf("isRecordExists returned false for an existing record")
	}

	nonExistentBookmark := &database.Bookmark{
		URL: "https://non_existent.com",
	}
	exists = r.RecordExists(nonExistentBookmark, "test_table")
	if exists {
		t.Errorf("isRecordExists returned true for a non-existent record")
	}
}

func TestUpdateRecordSuccess(t *testing.T) {
	var tableName string = "test_table"
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	bookmark := getValidBookmark()

	q := fmt.Sprintf("INSERT INTO %s (url) VALUES (?)", tableName)
	result, err := db.Exec(q, bookmark.URL)
	if err != nil {
		t.Errorf("Error inserting bookmark: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Errorf("Error getting last insert ID: %v", err)
	}
	bookmark.ID = int(id)

	_, err = r.UpdateRecord(&bookmark, tableName)
	if err != nil {
		t.Error(err)
	}
	q = fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tableName)
	row := db.QueryRow(q, bookmark.ID)
	var b database.Bookmark
	err = row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at)
	if err != nil {
		t.Errorf("Error scanning row: %v", err)
	}
	if b.ID != bookmark.ID {
		t.Errorf("Error updating bookmark: %v", database.ErrUpdateFailed)
	}
}

func TestUpdateRecordError(t *testing.T) {
	var tableName string = "test_table"
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	_, err := r.UpdateRecord(&database.Bookmark{}, tableName)
	if err == nil {
		t.Error("UpdateRecord did not return an error for an invalid record")
	}
}

func TestGetRecordByID(t *testing.T) {
	var tableName string = "test_table"
	db, r := setupTestDB(t)
	defer teardownTestDB(db)

	b := getValidBookmark()
	inserted, err := r.InsertRecord(&b, tableName)
	if err != nil {
		t.Fatal(err)
	}

	record, err := r.GetRecordByID(inserted.ID, tableName)
	if err != nil {
		t.Errorf("Error getting bookmark by ID: %v", err)
	}

	if record.ID != inserted.ID || record.URL != inserted.URL {
		t.Errorf("Unexpected bookmark retrieved: got %+v, expected %+v", record, inserted)
	}
}
