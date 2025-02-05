package repo

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/slice"
)

func TestInsert(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ts := r.Cfg.Tables
	tableExists, err := r.tableExists(ts.Main)
	assert.True(t, tableExists, "table %s does not exist", ts.Main)
	assert.NoError(t, err, "failed to check if table %s exists", ts.Main)
	// insert a record
	record := testSingleBookmark()
	err = r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, ts.Main, ts.RecordsTags, record)
	})
	assert.NoError(t, err, "failed to insert record into table %s", ts.Main)
}

func TestDeleteAll(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	tt := r.Cfg.Tables
	ts := []Table{
		tt.Main,
		tt.Deleted,
		tt.Tags,
		tt.RecordsTags,
		tt.RecordsTagsDeleted,
	}
	bs := testSliceBookmarks()
	ctx := context.Background()
	err := r.insertBulk(ctx, tt.Main, tt.RecordsTags, bs)
	assert.NoError(t, err)
	assert.NoError(t, r.deleteAll(ctx, ts...), "expected no error when deleting all records")
	b := bs.Item(0)
	_, err = r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.Error(t, err, "expected error when getting bookmark by ID, got nil")
}

func TestDeleteRecordBulk(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// insert bookmarks
	bs := testSliceBookmarks()
	ctx := context.Background()
	err := r.insertBulk(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, bs)
	assert.NoError(t, err, "insertBulk returned an error: %v", err)

	// verify that the record was inserted successfully
	newRows := slice.New[Row]()
	err = r.Records(r.Cfg.Tables.Main, newRows)
	assert.NoError(t, err, "failed to retrieve records")
	assert.Equal(t, bs.Len(), newRows.Len(), "expected %d records, got %d", bs.Len(), newRows.Len())

	// delete the record and verify that it was deleted successfully
	ids := slice.New[int]()
	newRows.ForEach(func(r Row) {
		ids.Append(r.ID)
	})
	assert.Equal(t, ids.Len(), newRows.Len(), "expected %d IDs, got %d", ids.Len(), newRows.Len())

	// test the deletion of a valid record
	err = r.deleteBulk(ctx, r.Cfg.Tables.Main, ids)
	assert.NoError(t, err, "deleteBulk returned an error: %v", err)

	// test no records found err
	emptyRows := slice.New[Row]()
	err = r.Records(r.Cfg.Tables.Main, emptyRows)
	assert.ErrorIs(t, err, ErrRecordNotFound, "expected ErrRecordNotFound, got %v", err)
	assert.Equal(t, 0, emptyRows.Len(), "expected 0 records, got %d", emptyRows.Len())
}

func TestByURL(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	err := r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	})
	assert.NoError(t, err)
	record, err := r.ByURL(r.Cfg.Tables.Main, b.URL)
	assert.NoError(t, err, "failed to retrieve bookmark by URL")
	assert.Equal(t, b.URL, record.URL, "expected bookmark URL %s, got %s", b.URL, b.URL)
}

func TestByID(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	bs := testSliceBookmarks()
	ctx := context.Background()
	err := r.insertBulk(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, bs)
	assert.NoError(t, err, "insertBulk returned an error: %v", err)
	recordID := 3
	record, err := r.ByID(r.Cfg.Tables.Main, recordID)
	assert.NoError(t, err, "failed to retrieve bookmark by ID")
	assert.Equal(t, record.ID, recordID, "expected bookmark ID %d, got %d", recordID, record.ID)
	assert.Equal(t, record.ID, recordID, "expected record ID %d, got %d", record.ID, recordID)
}

func TestDuplicateErr(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.NoError(t, err)
	err = r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.Error(t, err, "expected error when inserting duplicate record, got nil")
}

func TestGetRecordsByQuery(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	expectedRecords := 2

	b := testSingleBookmark()
	ctx := context.Background()
	_ = r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	b.URL = "https://www.example2.com"
	_ = r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	b.URL = "https://www.another.com"

	bs := slice.New[Row]()
	err := r.ByQuery(r.Cfg.Tables.Main, "example", bs)
	assert.NoError(t, err, "ByQuery returned an error: %v", err)
	assert.Equal(t, expectedRecords, bs.Len(), "%d records, got %d", expectedRecords, bs.Len())

	var count int
	err = r.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", r.Cfg.Tables.Main)).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, count, bs.Len(), "expected %d records, got %d", bs.Len(), count)
}

func TestRecordIsValid(t *testing.T) {
	validBookmark := testSingleBookmark()
	err := bookmark.Validate(validBookmark)
	assert.NoError(t, err, "expected valid bookmark to be valid")
	// invalid bookmark
	invalidBookmark := testSingleBookmark()
	invalidBookmark.Title = ""
	invalidBookmark.URL = ""
	err = bookmark.Validate(invalidBookmark)
	assert.Error(t, err, "expected invalid bookmark to be invalid")
}

func TestHasRecord(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.NoError(t, err, "insertInto returned an error: %v", err)

	exists := r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	assert.True(t, exists, "r.HasRecord returned false for an existing record")

	nonExistentBookmark := testSingleBookmark()
	nonExistentBookmark.URL = "https://non_existent.com"
	exists = r.HasRecord(r.Cfg.Tables.Main, "url", nonExistentBookmark.URL)
	assert.False(t, exists, "r.HasRecord returned true for a non-existent record")
}

func TestRollback(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	})
	assert.NoError(t, err, "Transaction failed")

	newB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark")
	assert.Equal(t, b.ID, newB.ID)
	assert.Equal(t, b.URL, newB.URL)
	err = r.execTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteInTx(tx, r.Cfg.Tables.Main, b); err != nil {
			return err
		}

		return ErrCommit
	})
	assert.Error(t, err, "Expected an error but got nil")
	// check if the record was deleted
	deletedB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark")
	assert.NotNil(t, deletedB, "Bookmark was not deleted")
}

func TestUpdate(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	originalB := testSingleBookmark()
	ctx := context.Background()
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, originalB)
	})
	assert.NoError(t, err, "Transaction failed")

	originalB.Desc = "new description"
	originalB.Title = "new title"
	var newB *Row
	newB, err = r.Update(originalB)
	assert.NoError(t, err, "Failed to update bookmark")
	assert.Equal(t, originalB.ID, newB.ID)
	assert.Equal(t, originalB.URL, newB.URL)
	assert.Equal(t, originalB.Desc, newB.Desc)
	assert.Equal(t, originalB.Title, newB.Title)

	// update URL
	originalB.URL = "https://newurl.com"
	newB, err = r.UpdateURL(originalB, originalB)
	assert.NoError(t, err, "Failed to update bookmark")
	assert.Equal(t, originalB.ID, newB.ID)
	assert.Equal(t, originalB.URL, newB.URL)
	assert.Equal(t, originalB.Desc, newB.Desc)
	assert.Equal(t, originalB.Title, newB.Title)
}

func TestDeleteAndReorder(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	t.Skip("not implemented")
}
