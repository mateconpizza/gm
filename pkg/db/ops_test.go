package db

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestDropRepository(t *testing.T) {
	t.Parallel()
	const n = 10
	r := testPopulatedDB(t, n)
	defer teardownthewall(r.DB)

	b, err := r.ByID(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error retrieving bookmark: %v", err)
	}
	if b == nil {
		t.Fatal("expected bookmark to exist, got nil")
	}

	err = drop(t.Context(), r)
	if err != nil {
		t.Fatalf("failed to drop repository: %v", err)
	}

	b, err = r.ByID(t.Context(), 1)
	if b != nil {
		t.Errorf("expected nil bookmark after drop, got: %+v", b)
	}
	if err == nil {
		t.Fatal("expected error after drop, got nil")
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("expected error to contain %q, got %q", ErrRecordNotFound.Error(), err.Error())
	}
}

func TestRecordIDs(t *testing.T) {
	t.Parallel()
	const want = 10
	r := testPopulatedDB(t, want)
	defer teardownthewall(r.DB)

	// get initial records
	bs, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("AllPtr failed: %v", err)
	}
	if len(bs) != want {
		t.Fatalf("expected %d records, got %d", want, len(bs))
	}

	// delete records at indices 1, 2, 5 (ids 2, 3, 6)
	toDelete := []*bookmark.Bookmark{bs[1], bs[2], bs[5]}
	if err := r.DeleteMany(t.Context(), toDelete); err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	// verify deletion - should have 7 records left
	remaining, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("All after deletion failed: %v", err)
	}
	if len(remaining) != 7 {
		t.Fatalf("expected 7 records after deletion, got %d", len(remaining))
	}

	// extract and verify current ids
	currentIDs := extractIDs(remaining)
	expectedAfterDelete := []int{1, 4, 5, 7, 8, 9, 10}
	if !reflect.DeepEqual(currentIDs, expectedAfterDelete) {
		t.Fatalf("after deletion, expected IDs %v, got %v", expectedAfterDelete, currentIDs)
	}

	if err := r.ReorderIDs(t.Context()); err != nil {
		t.Fatalf("ReorderIDs failed: %v", err)
	}

	// verify reordering - ids should be 1-7 consecutively
	reordered, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("AllPtr after reordering failed: %v", err)
	}
	if len(reordered) != 7 {
		t.Fatalf("expected 7 records after reordering, got %d", len(reordered))
	}

	reorderedIDs := extractIDs(reordered)
	expectedReordered := []int{1, 2, 3, 4, 5, 6, 7}
	if !reflect.DeepEqual(reorderedIDs, expectedReordered) {
		t.Fatalf("after reordering, expected IDs %v, got %v", expectedReordered, reorderedIDs)
	}
}

func extractIDs(bookmarks []*bookmark.Bookmark) []int {
	ids := make([]int, len(bookmarks))
	for i, b := range bookmarks {
		ids[i] = b.ID
	}
	return ids
}
