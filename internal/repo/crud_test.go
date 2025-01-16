package repo

import (
	"errors"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/slice"
)

func TestInsertInto(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := getBookmark()
	err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b)
	if err != nil {
		t.Fatal(err)
	}

	allRecords := slice.New[Row]()
	if err := r.Records(r.Cfg.Tables.Main, allRecords); err != nil {
		t.Fatal(err)
	}

	if allRecords.Len() != 1 {
		t.Errorf("Expected 1 record, got %d", allRecords.Len())
	}

	newB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	if err != nil {
		t.Fatal(err)
	}

	if newB.ID != b.ID || newB.URL != b.URL {
		t.Errorf("InsertInto: Unexpected bookmark retrieved: got %+v, expected %+v", newB, b)
	}

	duplicate := b

	if err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, duplicate); err == nil {
		t.Error("InsertInto did not return an error for a duplicate record")
	}

	// Insert an invalid record
	invalidBookmark := &Row{}
	if err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, invalidBookmark); err == nil {
		t.Error("InsertInto did not return an error for an invalid record")
	}
}

func TestInsertRecordsBulk(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	bs := getValidBookmarks()
	err := r.insertBulk(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, bs)
	if err != nil {
		t.Fatal(err)
	}

	n := bs.Len()
	var count int
	err = r.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", r.Cfg.Tables.Main)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != n {
		t.Errorf("Expected %d records, got %d", n, count)
	}
}

func TestDeleteAll(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	bs := getValidBookmarks()
	err := r.insertBulk(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, bs)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.deleteAll(r.Cfg.Tables.Main); err != nil {
		t.Fatal(err)
	}
	if err := r.deleteAll(r.Cfg.Tables.RecordsTags); err != nil {
		t.Fatal(err)
	}
	if err := r.deleteAll(r.Cfg.Tables.Tags); err != nil {
		t.Fatal(err)
	}

	b := bs.Item(0)
	if _, err := r.ByID(r.Cfg.Tables.Main, b.ID); err == nil {
		t.Errorf("TestDeleteAll: expected error when getting bookmark by ID, got nil")
	}
}

func TestDeleteAndReorder(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	t.Skip("not implemented")
}

func TestDeleteRecordBulk(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// insert bookmarks
	bs := getValidBookmarks()
	if err := r.insertBulk(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, bs); err != nil {
		t.Fatal(err)
	}

	// verify that the record was inserted successfully
	newRows := slice.New[Row]()
	if err := r.Records(r.Cfg.Tables.Main, newRows); err != nil {
		t.Fatal(err)
	}

	if newRows.Len() != bs.Len() {
		t.Errorf("Unexpected number of bookmarks retrieved: got %d, expected %d", newRows.Len(), 2)
	}

	// delete the record and verify that it was deleted successfully
	ids := slice.New[int]()
	newRows.ForEach(func(r Row) {
		ids.Append(&r.ID)
	})

	if ids.Len() != newRows.Len() {
		t.Errorf(
			"Unexpected number of IDs retrieved: got %d, expected %d",
			ids.Len(),
			newRows.Len(),
		)
	}

	// test the deletion of a valid record
	err := r.deleteBulk(r.Cfg.Tables.Main, ids)
	if err != nil {
		t.Errorf("DeleteRecordBulk returned an error: %v", err)
	}

	emptyRows := slice.New[Row]()
	err = r.Records(r.Cfg.Tables.Main, emptyRows)
	if !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("DeleteRecordBulk did not return '%v'", ErrRecordNotFound)
	}

	if emptyRows.Len() != 0 {
		t.Errorf(
			"Unexpected number of bookmarks retrieved: got %d, expected %d",
			emptyRows.Len(),
			0,
		)
	}
}

func TestByURL(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := getBookmark()
	if err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b); err != nil {
		t.Fatal(err)
	}

	_, err := r.ByURL(r.Cfg.Tables.Main, b.URL)
	if err != nil {
		t.Error(err)
	}
}

func TestUpdateRecord(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := getBookmark()

	tables := r.Cfg.Tables

	if err := r.InsertInto(tables.Main, tables.RecordsTags, tables.Tags, b); err != nil {
		t.Fatal(err)
	}

	newTags := "tag1,anotherTag"
	newDesc := "updated description"
	b.Tags = newTags
	b.Desc = newDesc

	bUpdated, err := r.Update(r.Cfg.Tables.Main, b)
	if err == nil {
		t.Errorf("Err updating bookmark: %v", err)
	}

	if bUpdated.Tags != newTags {
		t.Errorf("Err updating bookmark. Expected Tags: '%v', got '%v'", newTags, bUpdated.Tags)
	}

	if bUpdated.Desc != newDesc {
		t.Errorf("Err updating bookmark. Expected Desc: '%v', got '%v'", newDesc, bUpdated.Desc)
	}
}

func TestGetRecordByID(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := getBookmark()

	if err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b); err != nil {
		t.Fatal(err)
	}

	record, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	if err != nil {
		t.Errorf("Error getting bookmark by ID: %v", err)
	}

	if record.ID != b.ID || record.URL != b.URL {
		t.Errorf("Unexpected bookmark retrieved: got %+v, expected %+v", record, b)
	}
}

func TestGetRecordsByQuery(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	expectedRecords := 2

	b := getBookmark()
	_ = r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b)
	b.URL = "https://www.example2.com"
	_ = r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b)
	b.URL = "https://www.another.com"

	bs := slice.New[Row]()
	if err := r.ByQuery(r.Cfg.Tables.Main, "example", bs); err != nil {
		t.Errorf("Error getting bookmarks by query: %v", err)
	}

	n := bs.Len()

	if n != expectedRecords {
		t.Errorf("Unexpected number of bookmarks retrieved: got %d, expected %d", n, 2)
	}

	var count int
	err := r.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", r.Cfg.Tables.Main)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != n {
		t.Errorf("Expected %d records, got %d", n, count)
	}
}

func TestRecordIsValid(t *testing.T) {
	validBookmark := getBookmark()
	if err := bookmark.Validate(validBookmark); err != nil {
		t.Errorf("TestBookmarkIsValid: expected valid bookmark to be valid")
	}

	invalidBookmark := getBookmark()
	invalidBookmark.Title = ""
	invalidBookmark.URL = ""

	if err := bookmark.Validate(invalidBookmark); err == nil {
		t.Errorf("TestBookmarkIsValid: expected invalid bookmark to be invalid")
	}
}

func TestHasRecord(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := getBookmark()
	err := r.InsertInto(r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b)
	if err != nil {
		t.Fatal(err)
	}

	exists := r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	if !exists {
		t.Errorf("isRecordExists returned false for an existing record")
	}

	nonExistentBookmark := getBookmark()
	nonExistentBookmark.URL = "https://non_existent.com"
	exists = r.HasRecord(r.Cfg.Tables.Main, "url", nonExistentBookmark.URL)
	if exists {
		t.Errorf("isRecordExists returned true for a non-existent record")
	}
}
