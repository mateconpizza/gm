//nolint:paralleltest //test
package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"

	"github.com/haaag/gm/internal/slice"
)

func TestInsertOne(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	mainTable := schemaMain.name
	tableExists, err := r.tableExists(mainTable)
	assert.True(t, tableExists, "table %s does not exist", mainTable)
	assert.NoError(t, err, "failed to check if table %s exists", mainTable)
	// insert a record
	record := testSingleBookmark()
	err = r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, record)
	})
	assert.NoError(t, err, "failed to insert record into table %s", mainTable)
}

func TestInsertMany(t *testing.T) {
	t.Skip("not implemented yet")
}

func TestDeleteOne(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)

	b, err := r.ByID(1)
	assert.NoError(t, err)
	err = r.DeleteOne(context.Background(), b.URL)
	assert.NoError(t, err)
	// check if the record was deleted
	_, err = r.ByID(1)
	assert.Error(t, err, "expected error when getting bookmark by ID, got nil")
}

func TestDeleteMany(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	bsToInsert := testSliceBookmarks()
	ctx := context.Background()
	err := r.InsertMany(ctx, bsToInsert)
	assert.NoError(t, err)
	// check if the records were inserted
	bsInserted := slice.New[Row]()
	err = r.All(bsInserted)
	assert.NoError(t, err)
	assert.Len(t, *bsInserted.Items(), 10)
	// delete the records
	err = r.DeleteMany(ctx, bsToInsert)
	assert.NoError(t, err)
	// check if the records were deleted
	bsDeleted := slice.New[Row]()
	err = r.All(bsDeleted)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUpdateOne(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	oldB := testSingleBookmark()
	// insert record
	ctx := context.Background()
	err := r.InsertOne(ctx, oldB)
	assert.NoError(t, err)
	// copy bookmark
	newB := oldB
	assert.Equal(t, oldB, newB)
	// modify bookmark
	newDesc := "new description"
	newTags := "tagNew1,tagNew2,"
	newB.Tags = newTags
	newB.Desc = newDesc
	// update record in main table
	_, err = r.UpdateOne(ctx, newB, oldB)
	assert.NoError(t, err, "expected no error, got %v", err)
	// get record byID
	updateB, err := r.ByID(newB.ID)
	assert.NoError(t, err)
	// check if the record was updated
	assert.Equal(t, newB.ID, updateB.ID, "expected bookmark ID %d, got %d", oldB.ID, updateB.ID)
	assert.Equal(t, updateB.Desc, newB.Desc)
	assert.Equal(t, updateB.Tags, newB.Tags)
	assert.Equal(t, updateB.UpdatedAt, newB.UpdatedAt)
	assert.Equal(t, updateB.CreatedAt, oldB.CreatedAt)
	assert.Equal(t, updateB.Favorite, oldB.Favorite)
}

func TestAllRecords(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)
	// get all records
	bs := slice.New[Row]()
	err := r.All(bs)
	assert.NoError(t, err, "All returned an error: %v", err)
	assert.Len(t, *bs.Items(), 10, "expected %d records, got %d", 10, bs.Len())
}

func TestByID(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)
	// get all records
	bs := slice.New[Row]()
	err := r.All(bs)
	assert.NoError(t, err, "All returned an error: %v", err)
	assert.Len(t, *bs.Items(), 10, "expected %d records, got %d", 10, bs.Len())
	// get a specific record
	compareB := bs.Item(0)
	// get record byID
	record, err := r.ByID(compareB.ID)
	assert.NoError(t, err, "failed to retrieve bookmark by ID")
	assert.Equal(t, record.ID, compareB.ID)
	assert.Equal(t, record.URL, compareB.URL)
	assert.Equal(t, record.Desc, compareB.Desc)
	assert.Equal(t, record.Tags, compareB.Tags)
}

func TestByIDList(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)

	ids := []int{1, 4, 2, 5, 8}
	bs := slice.New[Row]()
	err := r.ByIDList(ids, bs)
	assert.NoError(t, err)
	assert.Len(t, *bs.Items(), 5)
	bs.ForEach(func(b Row) {
		assert.Contains(t, ids, b.ID)
	})
}

func TestByURL(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	err := r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
	assert.NoError(t, err)
	record, err := r.ByURL(b.URL)
	assert.NoError(t, err, "failed to retrieve bookmark by URL")
	assert.Equal(t, b.URL, record.URL, "expected bookmark URL %s, got %s", b.URL, b.URL)
}

func TestByTag(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)
	bs := slice.New[Row]()
	err := r.ByTag(context.Background(), "tag1", bs)
	assert.NoError(t, err)
	assert.Len(t, *bs.Items(), 1)
}

func TestByQuery(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	expectedRecords := 2

	b := testSingleBookmark()
	ctx := context.Background()
	_ = r.insertInto(ctx, b)
	b.URL = "https://www.example2.com"
	_ = r.insertInto(ctx, b)
	b.URL = "https://www.another.com"

	bs := slice.New[Row]()
	err := r.ByQuery("example", bs)
	assert.NoError(t, err, "ByQuery returned an error: %v", err)
	assert.Equal(t, expectedRecords, bs.Len(), "%d records, got %d", expectedRecords, bs.Len())

	var count int
	err = r.DB.QueryRow("SELECT COUNT(*) FROM bookmarks").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, count, bs.Len(), "expected %d records, got %d", bs.Len(), count)
}

func TestDuplicateErr(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, b)
	assert.NoError(t, err)
	err = r.insertInto(ctx, b)
	assert.Error(t, err, "expected error when inserting duplicate record, got nil")
}

func TestHasRecord(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.insertInto(ctx, b)
	assert.NoError(t, err, "insertInto returned an error: %v", err)

	_, exists := r.Has(b.URL)
	assert.True(t, exists, "r.HasRecord returned false for an existing record")

	nonExistentBookmark := testSingleBookmark()
	nonExistentBookmark.URL = "https://non_existent.com"
	_, exists = r.Has(nonExistentBookmark.URL)
	assert.False(t, exists, "r.HasRecord returned true for a non-existent record")
}

func TestRollback(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	b := testSingleBookmark()
	ctx := context.Background()
	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
	assert.NoError(t, err, "Transaction failed")

	newB, err := r.ByID(b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark")
	assert.Equal(t, b.ID, newB.ID)
	assert.Equal(t, b.URL, newB.URL)
	err = r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteOneTx(tx, b); err != nil {
			return err
		}

		return ErrCommit
	})
	assert.Error(t, err, "Expected an error but got nil")
	// check if the record was deleted
	deletedB, err := r.ByID(b.ID)
	assert.NoError(t, err, "Failed to retrieve bookmark")
	assert.NotNil(t, deletedB, "Bookmark was not deleted")
}

func TestDeleteAll(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ts := []Table{
		schemaMain.name,
		schemaTags.name,
		schemaRelation.name,
	}
	bs := testSliceBookmarks()
	ctx := context.Background()
	err := r.insertBulk(ctx, bs)
	assert.NoError(t, err)
	assert.NoError(t, r.deleteAll(ctx, ts...), "expected no error when deleting all records")
	b := bs.Item(0)
	_, err = r.ByID(b.ID)
	assert.Error(t, err, "expected error when getting bookmark by ID, got nil")
}

func TestRecordIDs(t *testing.T) {
	r := testPopulatedDB(t)
	defer teardownthewall(r.DB)
	// get all records
	bs := slice.New[Row]()
	err := r.All(bs)
	assert.NoError(t, err, "All returned an error: %v", err)
	assert.Len(t, *bs.Items(), 10, "expected %d records, got %d", 10, bs.Len())
	// delete some records
	bsToDelete := slice.New[Row]()
	bsToDelete.Append(bs.Item(1), bs.Item(3), bs.Item(5))
	ctx := context.Background()
	err = r.DeleteMany(ctx, bsToDelete)
	assert.NoError(t, err, "DeleteMany returned an error: %v", err)
	// check if the records were deleted
	bsDeleted := slice.New[Row]()
	err = r.All(bsDeleted)
	assert.NoError(t, err, "All returned an error: %v", err)
	assert.Len(t, *bsDeleted.Items(), 7, "expected %d records, got %d", 7, bsDeleted.Len())
	// get the IDs of the remaining records
	var ids []int
	bsDeleted.ForEach(func(b Row) {
		ids = append(ids, b.ID)
	})
	// IDs
	currentIDs := []int{1, 3, 5, 7, 8, 9, 10}
	assert.Len(t, currentIDs, 7, "expected %d IDs, got %d", 7, len(ids))
	assert.Equal(t, currentIDs, ids, "expected IDs to be %v, got %v", currentIDs, ids)
	// reorder the IDs
	err = r.ReorderIDs(ctx)
	assert.NoError(t, err, "ReorderIDs returned an error: %v", err)
	// comparate new ordered IDs
	orderedBs := slice.New[Row]()
	err = r.All(orderedBs)
	assert.NoError(t, err, "All returned an error: %v", err)
	assert.Len(t, *orderedBs.Items(), 7, "expected %d records, got %d", 7, orderedBs.Len())
	var orderedIDs []int
	orderedBs.ForEach(func(b Row) {
		orderedIDs = append(orderedIDs, b.ID)
	})
	// ordered IDs
	expectedIDs := []int{1, 2, 3, 4, 5, 6, 7}
	assert.Equal(
		t,
		expectedIDs,
		orderedIDs,
		"expected IDs to be %v, got %v",
		expectedIDs,
		orderedIDs,
	)
	assert.ElementsMatch(
		t,
		expectedIDs,
		orderedIDs,
		"expected IDs to be %v, got %v",
		expectedIDs,
		orderedIDs,
	)
}
