package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestDropRepository(t *testing.T) {
	t.Parallel()
	const n = 10
	r := testPopulatedDB(t, n)

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

func TestNewBackup(t *testing.T) {
	t.Parallel()
	fixedTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func(t *testing.T) (*SQLite, string)
		now     time.Time
		wantErr error
	}{
		{
			"valid_backup",
			func(t *testing.T) (*SQLite, string) {
				t.Helper()
				r := testPopulatedDB(t, 5)
				return r, t.TempDir()
			},
			fixedTime,
			nil,
		},
		{
			"empty_dest_root",
			func(t *testing.T) (*SQLite, string) {
				t.Helper()
				r := testPopulatedDB(t, 5)
				return r, ""
			},
			fixedTime,
			ErrDBEmptyPath, // was nil
		},
		{
			"backup_already_exists",
			func(t *testing.T) (*SQLite, string) {
				t.Helper()
				r := testPopulatedDB(t, 5)
				tempDir := t.TempDir()
				// pre-create the file so the second call hits ErrBackupExists
				destDSN := fmt.Sprintf("%s_%s", fixedTime.Format(defaultDateFormat), r.Name())
				destPath := filepath.Join(tempDir, destDSN)
				if err := os.WriteFile(destPath, []byte{}, 0o644); err != nil {
					t.Fatalf("failed to pre-create backup file: %v", err)
				}
				return r, tempDir
			},
			fixedTime,
			ErrBackupExists,
		},
		{
			"empty_db_still_backs_up",
			func(t *testing.T) (*SQLite, string) {
				t.Helper()
				r := testPopulatedDB(t, 0)
				return r, t.TempDir()
			},
			fixedTime,
			nil,
		},
		{
			"backup_path_uses_correct_timestamp",
			func(t *testing.T) (*SQLite, string) {
				t.Helper()
				r := testPopulatedDB(t, 1)
				return r, t.TempDir()
			},
			time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, destRoot := tt.setup(t)
			defer teardownthewall(r.DB)

			got, err := r.newBackup(t.Context(), destRoot, tt.now)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("newBackup() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("newBackup() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("newBackup() unexpected error: %v", err)
			}

			// verify the file exists
			if _, err := os.Stat(got); err != nil {
				t.Fatalf("backup file not found at %q: %v", got, err)
			}

			// verify path is built correctly from timestamp and db name
			want := filepath.Join(destRoot, fmt.Sprintf("%s_%s", tt.now.Format(defaultDateFormat), r.Name()))
			if got != want {
				t.Fatalf("newBackup() path = %q; want %q", got, want)
			}
		})
	}
}
