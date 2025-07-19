//nolint:paralleltest,wsl,funlen //test
package db

import (
	"errors"
	"slices"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestInsertOne(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// verify table exists
	mainTable := schemaMain.name
	tableExists, err := r.tableExists(mainTable)
	if err != nil {
		t.Fatalf("failed to check if table %s exists: %v", mainTable, err)
	}
	if !tableExists {
		t.Fatalf("table %s does not exist", mainTable)
	}

	// insert a record
	record := testSingleBookmark()
	err = r.withTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, record)
	})
	if err != nil {
		t.Fatalf("failed to insert record into table %s: %v", mainTable, err)
	}
}

func TestInsertMany(t *testing.T) {
	t.Skip("not implemented yet")
}

func TestDeleteOne(t *testing.T) {
	t.Parallel()
	r := testPopulatedDB(t, 10)
	defer teardownthewall(r.DB)

	b, err := r.ByID(1)
	if err != nil {
		t.Fatalf("Failed to get bookmark by ID: %v", err)
	}

	err = r.DeleteByURL(t.Context(), b.URL)
	if err != nil {
		t.Fatalf("Failed to delete bookmark by URL: %v", err)
	}

	// check if the record was deleted
	_, err = r.ByID(1)
	if err == nil {
		t.Error("Expected an error when getting bookmark by ID, but got nil (record was not deleted)")
	}
}

func TestDeleteMany(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		bookmarksToInsert int
		bookmarksToDelete int
		expectErrOnInsert bool
		expectErrOnDelete bool
		expectedError     error
	}{
		{
			name:              "Successfully inserts and deletes 10 bookmarks",
			bookmarksToInsert: 10,
			expectErrOnInsert: false,
			expectErrOnDelete: false,
		},
		{
			name:              "Deletes zero bookmarks (empty slice)",
			bookmarksToInsert: 0,
			bookmarksToDelete: 0,
			expectErrOnInsert: false,
			expectErrOnDelete: true,
			expectedError:     ErrRecordIDNotProvided,
		},
		{
			name:              "Deletes some but not all existing bookmarks",
			bookmarksToInsert: 10,
			bookmarksToDelete: 5,
			expectErrOnInsert: false,
			expectErrOnDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupTestDB(t)
			defer teardownthewall(r.DB)

			bsToInsert := testSliceBookmarks(tt.bookmarksToInsert)

			err := r.InsertMany(t.Context(), bsToInsert)
			if (err != nil) != tt.expectErrOnInsert {
				t.Errorf("InsertMany() error = %v, expectErrOnInsert %v", err, tt.expectErrOnInsert)
				return
			}
			if tt.expectErrOnInsert {
				return
			}

			// check if the records were inserted
			wantAfterInsert := tt.bookmarksToInsert
			inserted, err := r.All()
			if err != nil {
				t.Fatalf("Failed to get all bookmarks after insertion: %v", err)
			}
			if len(inserted) != wantAfterInsert {
				t.Errorf("After insert: Expected %d bookmarks, got %d", wantAfterInsert, len(inserted))
			}

			// Delete the records
			err = r.DeleteMany(t.Context(), bsToInsert)
			if (err != nil) != tt.expectErrOnDelete {
				t.Errorf("DeleteMany() error = %v, expectErrOnDelete %v", err, tt.expectErrOnDelete)
				return
			}
			if tt.expectErrOnDelete {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf("DeleteMany() error = %v, expected error = %v", err, tt.expectedError)
				}
			}

			// Check if the records were deleted
			wantAfterDelete := 0 // For this specific test case, we expect all to be deleted
			deleted, err := r.All()
			if err != nil {
				t.Fatalf("Failed to get all bookmarks after deletion: %v", err)
			}
			if len(deleted) != wantAfterDelete {
				t.Errorf("After delete: Expected %d bookmarks, got %d", wantAfterDelete, len(deleted))
			}
		})
	}
}

func TestUpdateOne(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// Insert initial record
	oldB := testSingleBookmark()
	if err := r.InsertOne(t.Context(), oldB); err != nil {
		t.Fatalf("failed to insert bookmark: %v", err)
	}

	// Create updated bookmark with new values
	newB := *oldB // Create a copy by dereferencing and re-referencing
	newB.Checksum = "new-checksum"
	newB.Tags = "tagNew1,tagNew2,"
	newB.Desc = "new description"

	// Update the record
	if err := r.Update(t.Context(), &newB, oldB); err != nil {
		t.Fatalf("failed to update bookmark: %v", err)
	}

	// Retrieve the updated record
	updatedB, err := r.ByID(newB.ID)
	if err != nil {
		t.Fatalf("failed to retrieve updated bookmark: %v", err)
	}

	// Verify the update
	if updatedB.ID != newB.ID {
		t.Errorf("expected ID %d, got %d", newB.ID, updatedB.ID)
	}
	if updatedB.Desc != newB.Desc {
		t.Errorf("expected description %q, got %q", newB.Desc, updatedB.Desc)
	}
	if updatedB.Tags != newB.Tags {
		t.Errorf("expected tags %q, got %q", newB.Tags, updatedB.Tags)
	}
	if updatedB.UpdatedAt != newB.UpdatedAt {
		t.Errorf("expected UpdatedAt %v, got %v", newB.UpdatedAt, updatedB.UpdatedAt)
	}
	if updatedB.CreatedAt != oldB.CreatedAt {
		t.Errorf("expected CreatedAt %v, got %v", oldB.CreatedAt, updatedB.CreatedAt)
	}
	if updatedB.Favorite != oldB.Favorite {
		t.Errorf("expected Favorite %v, got %v", oldB.Favorite, updatedB.Favorite)
	}
}

func TestAllRecords(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	// get bs records
	bs, err := r.All()
	got := len(bs)
	if err != nil {
		t.Fatalf("failed to get all bookmarks: %v", err)
	}

	if got != want {
		t.Errorf("expected %d records, got %d", want, got)
	}
}

func TestByID(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	// Get all records to verify setup
	all, err := r.All()
	if err != nil {
		t.Fatalf("All() failed: %v", err)
	}
	if len(all) != want {
		t.Fatalf("expected %d records, got %d", want, len(all))
	}

	// Test retrieving a specific record by ID
	expected := all[0]
	record, err := r.ByID(expected.ID)
	if err != nil {
		t.Fatalf("ByID(%d) failed: %v", expected.ID, err)
	}

	// Verify the retrieved record matches expected
	if record.ID != expected.ID {
		t.Errorf("expected ID %d, got %d", expected.ID, record.ID)
	}
	if record.URL != expected.URL {
		t.Errorf("expected URL %q, got %q", expected.URL, record.URL)
	}
	if record.Desc != expected.Desc {
		t.Errorf("expected description %q, got %q", expected.Desc, record.Desc)
	}
	if record.Tags != expected.Tags {
		t.Errorf("expected tags %q, got %q", expected.Tags, record.Tags)
	}
}

func TestByIDList(t *testing.T) {
	t.Parallel()
	r := testPopulatedDB(t, 10)
	defer teardownthewall(r.DB)

	// Test retrieving multiple records by ID list
	ids := []int{1, 4, 2, 5, 8}
	bookmarks, err := r.ByIDList(ids)
	if err != nil {
		t.Fatalf("ByIDList(%v) failed: %v", ids, err)
	}

	// Verify correct number of records returned
	if len(bookmarks) != len(ids) {
		t.Fatalf("expected %d bookmarks, got %d", len(ids), len(bookmarks))
	}

	// Verify all returned bookmarks have IDs from the requested list
	for _, bookmark := range bookmarks {
		if !slices.Contains(ids, bookmark.ID) {
			t.Errorf("bookmark ID %d not in requested list %v", bookmark.ID, ids)
		}
	}
}

func TestByURL(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// Insert a test bookmark
	b := testSingleBookmark()
	err := r.withTx(t.Context(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
	if err != nil {
		t.Fatalf("failed to insert bookmark: %v", err)
	}

	// Retrieve bookmark by URL
	record, err := r.ByURL(b.URL)
	if err != nil {
		t.Fatalf("ByURL(%q) failed: %v", b.URL, err)
	}

	// Verify the retrieved record has the correct URL
	if record.URL != b.URL {
		t.Errorf("expected URL %q, got %q", b.URL, record.URL)
	}
}

func TestByTag(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	bookmarks, err := r.ByTag(t.Context(), "tag1")
	if err != nil {
		t.Fatalf("ByTag failed: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Errorf("expected 1 bookmark with tag1, got %d", len(bookmarks))
	}
}

func TestByQuery(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ctx := t.Context()

	// Insert test bookmarks
	b := testSingleBookmark()
	if err := r.insertInto(ctx, b); err != nil {
		t.Fatalf("failed to insert first bookmark: %v", err)
	}

	b.URL = "https://www.example2.com"
	if err := r.insertInto(ctx, b); err != nil {
		t.Fatalf("failed to insert second bookmark: %v", err)
	}

	b.URL = "https://www.another.com"
	if err := r.insertInto(ctx, b); err != nil {
		t.Fatalf("failed to insert third bookmark: %v", err)
	}

	// Search for bookmarks containing "example"
	results, err := r.ByQuery("example")
	if err != nil {
		t.Fatalf("ByQuery failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for query 'example', got %d", len(results))
	}

	// Verify total count in database
	var count int
	err = r.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookmarks").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count records: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 total records, got %d", count)
	}
}

func TestDuplicateErr(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ctx := t.Context()

	b := testSingleBookmark()

	// Insert bookmark successfully
	if err := r.insertInto(ctx, b); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Attempt to insert duplicate should fail
	if err := r.insertInto(ctx, b); err == nil {
		t.Error("expected error when inserting duplicate record, got nil")
	}
}

func TestHasRecord(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ctx := t.Context()

	b := testSingleBookmark()
	if err := r.insertInto(ctx, b); err != nil {
		t.Fatalf("insertInto failed: %v", err)
	}

	// Test existing record
	_, exists := r.Has(b.URL)
	if !exists {
		t.Error("Has() returned false for an existing record")
	}

	// Test non-existent record
	_, exists = r.Has("https://non_existent.com")
	if exists {
		t.Error("Has() returned true for a non-existent record")
	}
}

func TestRollback(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	ctx := t.Context()

	b := testSingleBookmark()

	// Insert bookmark successfully
	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	// Verify bookmark was inserted
	insertedB, err := r.ByID(b.ID)
	if err != nil {
		t.Fatalf("failed to retrieve bookmark: %v", err)
	}
	if insertedB.ID != b.ID {
		t.Errorf("expected ID %d, got %d", b.ID, insertedB.ID)
	}
	if insertedB.URL != b.URL {
		t.Errorf("expected URL %q, got %q", b.URL, insertedB.URL)
	}

	// Attempt transaction that should rollback
	err = r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteOneTx(tx, b); err != nil {
			return err
		}
		return ErrCommit // Force rollback
	})
	if err == nil {
		t.Error("expected transaction to fail with ErrCommit, got nil")
	}

	// Verify bookmark still exists (rollback worked)
	stillExists, err := r.ByID(b.ID)
	if err != nil {
		t.Fatalf("failed to retrieve bookmark after rollback: %v", err)
	}
	if stillExists == nil {
		t.Error("bookmark was deleted despite rollback")
	}
}

func TestDeleteAll(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// Define tables to delete from
	tables := []Table{
		schemaMain.name,
		schemaTags.name,
		schemaRelation.name,
	}

	// Insert test data
	bookmarks := testSliceBookmarks(10)
	if err := r.insertBulkPtr(t.Context(), bookmarks); err != nil {
		t.Fatalf("failed to insert bulk bookmarks: %v", err)
	}

	// Delete all records from tables
	if err := r.deleteAll(t.Context(), tables...); err != nil {
		t.Fatalf("deleteAll failed: %v", err)
	}

	// Verify records were deleted by trying to retrieve one
	testBookmark := bookmarks[0]
	_, err := r.ByID(testBookmark.ID)
	if err == nil {
		t.Error("expected error when getting bookmark by ID after deleteAll, got nil")
	}
}

func TestRecordIDs(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	// get initial records
	bs, err := r.All()
	if err != nil {
		t.Fatalf("AllPtr failed: %v", err)
	}
	if len(bs) != want {
		t.Fatalf("expected %d records, got %d", want, len(bs))
	}

	// delete records at indices 1, 2, 5 (ids 2, 3, 6)
	toDelete := []*BookmarkModel{bs[1], bs[2], bs[5]}
	if err := r.DeleteMany(t.Context(), toDelete); err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	// verify deletion - should have 7 records left
	remaining, err := r.All()
	if err != nil {
		t.Fatalf("AllPtr after deletion failed: %v", err)
	}
	if len(remaining) != 7 {
		t.Fatalf("expected 7 records after deletion, got %d", len(remaining))
	}

	// extract and verify current ids
	currentIDs := extractIDs(remaining)
	expectedAfterDelete := []int{1, 4, 5, 7, 8, 9, 10}
	if !equalIntSlices(currentIDs, expectedAfterDelete) {
		t.Fatalf("after deletion, expected IDs %v, got %v", expectedAfterDelete, currentIDs)
	}

	// reorder ids
	if err := r.ReorderIDs(t.Context()); err != nil {
		t.Fatalf("ReorderIDs failed: %v", err)
	}

	// verify reordering - ids should be 1-7 consecutively
	reordered, err := r.All()
	if err != nil {
		t.Fatalf("AllPtr after reordering failed: %v", err)
	}
	if len(reordered) != 7 {
		t.Fatalf("expected 7 records after reordering, got %d", len(reordered))
	}

	reorderedIDs := extractIDs(reordered)
	expectedReordered := []int{1, 2, 3, 4, 5, 6, 7}
	if !equalIntSlices(reorderedIDs, expectedReordered) {
		t.Fatalf("after reordering, expected IDs %v, got %v", expectedReordered, reorderedIDs)
	}
}

func extractIDs(bookmarks []*BookmarkModel) []int {
	ids := make([]int, len(bookmarks))
	for i, b := range bookmarks {
		ids[i] = b.ID
	}
	return ids
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
