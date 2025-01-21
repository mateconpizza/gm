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

func TestInsertInto(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.NoError(t, err)

	allRecords := slice.New[Row]()
	err = r.Records(r.Cfg.Tables.Main, allRecords)
	assert.NoError(t, err)
	assert.Len(t, *allRecords.Items(), 1, "expected 1 record, got %d", allRecords.Len())

	newB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "failed to retrieve bookmark")
	assert.Equal(t, b.ID, newB.ID)
	assert.Equal(t, b.URL, newB.URL)

	duplicate := b

	err = r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, duplicate)
	assert.ErrorIs(t, err, ErrRecordDuplicate, "expected duplicated record error")

	// Insert an invalid record
	invalidBookmark := &Row{}
	err = r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, invalidBookmark)
	assert.Error(t, err, "should return an error for an invalid record")
}

func TestInsertRecordsBulk(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	bs := testSliceBookmarks()
	ctx := context.Background()
	assert.NoError(t, r.insertBulk(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, bs))

	n := bs.Len()
	var count int
	err := r.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", r.Cfg.Tables.Main)).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, n, count, "expected %d records, got %d", n, count)
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
		ids.Append(&r.ID)
	})
	assert.Equal(t, ids.Len(), newRows.Len(), "expected %d IDs, got %d", ids.Len(), newRows.Len())

	// test the deletion of a valid record
	err = r.deleteBulk(ctx, r.Cfg.Tables.Main, ids)
	assert.NoError(t, err, "deleteBulk returned an error: %v", err)

	// test no records found err
	emptyRows := slice.New[Row]()
	err = r.Records(r.Cfg.Tables.Main, emptyRows)
	assert.ErrorIs(t, err, ErrRecordNotFound, "expected ErrRecordNotFound, got %v", err)
	assert.Equal(t, emptyRows.Len(), 0, "expected 0 records, got %d", emptyRows.Len())
}

func TestByURL(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.NoError(t, err)
	_, err = r.ByURL(r.Cfg.Tables.Main, b.URL)
	assert.NoError(t, err)
}

func TestGetRecordByID(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()

	ctx := context.Background()
	err := r.insertInto(ctx, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, b)
	assert.NoError(t, err, "insertInto returned an error: %v", err)
	record, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "ByID returned an error: %v", err)
	assert.Equal(t, record.ID, b.ID, "expected record ID %d, got %d", b.ID, record.ID)
	assert.Equal(t, record.URL, b.URL, "expected record URL %s, got %s", b.URL, record.URL)
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
	assert.Equal(t, bs.Len(), expectedRecords, "%d records, got %d", expectedRecords, bs.Len())

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
		q := fmt.Sprintf("INSERT INTO %s (url, title, desc) VALUES (?, ?, ?)", r.Cfg.Tables.Main)
		result, err := tx.Exec(q, b.URL, b.Title, b.Desc)
		assert.NoError(t, err, "Failed to insert bookmark")

		newID, err := result.LastInsertId()
		assert.NoError(t, err, "Failed to retrieve last insert ID")

		b.ID = int(newID)

		return nil
	})
	assert.NoError(t, err, "Transaction failed")

	newB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark")
	assert.Equal(t, b.ID, newB.ID)
	assert.Equal(t, b.URL, newB.URL)

	err = r.execTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.Delete(ctx, r.Cfg.Tables.Main, b); err != nil {
			return err
		}

		return ErrCommit
	})
	assert.Error(t, err, "Expected an error but got nil")

	deletedB, err := r.ByID(r.Cfg.Tables.Main, b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark after rollback")
	assert.Equal(t, b.ID, deletedB.ID)
	assert.Equal(t, b.URL, deletedB.URL)
}

func TestDeleteAndReorder(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	t.Skip("not implemented")
}
