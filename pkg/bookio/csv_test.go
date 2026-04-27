package bookio

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestExportToCSV(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		bookmarks  []*bookmark.Bookmark
		fields     []string
		wantErr    error
		wantHeader []string
		wantRows   [][]string
	}{
		{
			name: "default_fields_nil_uses_csv_default_header",
			bookmarks: []*bookmark.Bookmark{
				{
					ID:        1,
					URL:       "https://example.com",
					Title:     "Example",
					Desc:      "A site",
					Notes:     "",
					CreatedAt: "2026-04-22T14:38:45Z",
					Favorite:  false,
				},
			},
			fields:     nil,
			wantHeader: CSVDefaultHeader,
			wantRows: [][]string{
				{"1", "https://example.com", "Example", "A site", "2026-04-22T14:38:45Z", "false", ""},
			},
		},
		{
			name: "custom_fields_subset_int_and_bool",
			bookmarks: []*bookmark.Bookmark{
				{
					ID:         7,
					URL:        "https://go.dev",
					Favorite:   true,
					VisitCount: 42,
				},
			},
			fields:     []string{"id", "url", "favorite", "visit_count"},
			wantHeader: []string{"id", "url", "favorite", "visit_count"},
			wantRows: [][]string{
				{"7", "https://go.dev", "true", "42"},
			},
		},
		{
			// no bookmark rows: only the header should be written.
			name:       "empty_bookmark_slice_writes_only_header",
			bookmarks:  []*bookmark.Bookmark{},
			fields:     []string{"id", "url", "title"},
			wantHeader: []string{"id", "url", "title"},
			wantRows:   [][]string{},
		},
		{
			// unknown field must return errinvalidfield before writing anything.
			name: "unknown_field_returns_err_invalid_field",
			bookmarks: []*bookmark.Bookmark{
				{ID: 1, URL: "https://example.com"},
			},
			fields:  []string{"id", "nonexistent_field"},
			wantErr: ErrInvalidField,
		},
		{
			// multiple rows with realistic timestamps; empty optional fields stay "".
			// also exercises is_active (bool) and status_code (int zero value).
			name: "multiple_bookmarks_with_timestamps",
			bookmarks: []*bookmark.Bookmark{
				{
					ID:        1,
					URL:       "https://alpha.io",
					Title:     "Alpha",
					CreatedAt: "2026-04-22T14:38:45Z",
					UpdatedAt: "2026-04-22T14:57:29Z",
					LastVisit: "",
					Favorite:  true,
					IsActive:  true,
				},
				{
					ID:        2,
					URL:       "https://beta.io",
					Title:     "Beta",
					CreatedAt: "2026-04-23T09:00:00Z",
					UpdatedAt: "2026-04-23T09:00:00Z",
					LastVisit: "2026-04-24T18:30:00Z",
					Favorite:  false,
					IsActive:  false,
				},
			},
			fields: []string{
				"id",
				"url",
				"title",
				"created_at",
				"updated_at",
				"last_visit",
				"favorite",
				"is_active",
			},
			wantHeader: []string{
				"id",
				"url",
				"title",
				"created_at",
				"updated_at",
				"last_visit",
				"favorite",
				"is_active",
			},
			wantRows: [][]string{
				{"1", "https://alpha.io", "Alpha", "2026-04-22T14:38:45Z", "2026-04-22T14:57:29Z", "", "true", "true"},
				{
					"2",
					"https://beta.io",
					"Beta",
					"2026-04-23T09:00:00Z",
					"2026-04-23T09:00:00Z",
					"2026-04-24T18:30:00Z",
					"false",
					"false",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer

			err := ExportToCSV(tt.bookmarks, &buf, tt.fields)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ExportToCSV() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ExportToCSV() error = %v; want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExportToCSV() unexpected error: %v", err)
			}

			records := parseCSV(t, &buf)

			if len(records) == 0 {
				t.Fatal("ExportToCSV() produced empty output, expected at least a header row")
			}

			assertHeader(t, records[0], tt.wantHeader)
			assertRows(t, records[1:], tt.wantRows)
		})
	}
}

func parseCSV(t *testing.T, buf *bytes.Buffer) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}
	return records
}

func assertHeader(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("header length = %d; want %d: got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("header[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func assertRows(t *testing.T, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("row count = %d; want %d", len(got), len(want))
	}
	for i, wantRow := range want {
		gotRow := got[i]
		if len(gotRow) != len(wantRow) {
			t.Fatalf("row[%d] length = %d; want %d", i, len(gotRow), len(wantRow))
		}
		for j := range wantRow {
			if gotRow[j] != wantRow[j] {
				t.Errorf("row[%d][%d] = %q; want %q", i, j, gotRow[j], wantRow[j])
			}
		}
	}
}
