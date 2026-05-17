package port

import (
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestDeduplicate(t *testing.T) {
	t.Parallel()
	bk := func(url string) *bookmark.Bookmark {
		return &bookmark.Bookmark{URL: url}
	}

	tests := []struct {
		name           string
		bs             []*bookmark.Bookmark
		existing       []*bookmark.Bookmark
		wantFresh      []*bookmark.Bookmark
		wantDuplicates []*bookmark.Bookmark
	}{
		{
			name:           "all_fresh",
			bs:             []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*bookmark.Bookmark{bk("https://c.com")},
			wantFresh:      []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*bookmark.Bookmark{},
		},
		{
			name:           "all_duplicates",
			bs:             []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantFresh:      []*bookmark.Bookmark{},
			wantDuplicates: []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
		},
		{
			name:           "mixed_fresh_and_duplicates",
			bs:             []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com"), bk("https://c.com")},
			existing:       []*bookmark.Bookmark{bk("https://b.com")},
			wantFresh:      []*bookmark.Bookmark{bk("https://a.com"), bk("https://c.com")},
			wantDuplicates: []*bookmark.Bookmark{bk("https://b.com")},
		},
		{
			name:           "empty_bs",
			bs:             []*bookmark.Bookmark{},
			existing:       []*bookmark.Bookmark{bk("https://a.com")},
			wantFresh:      []*bookmark.Bookmark{},
			wantDuplicates: []*bookmark.Bookmark{},
		},
		{
			name:           "empty_existing",
			bs:             []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*bookmark.Bookmark{},
			wantFresh:      []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*bookmark.Bookmark{},
		},
		{
			name:           "nil_entries_in_bs_are_skipped",
			bs:             []*bookmark.Bookmark{bk("https://a.com"), nil, bk("https://b.com")},
			existing:       []*bookmark.Bookmark{},
			wantFresh:      []*bookmark.Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*bookmark.Bookmark{},
		},
		{
			name:           "nil_entries_in_existing_are_skipped",
			bs:             []*bookmark.Bookmark{bk("https://a.com")},
			existing:       []*bookmark.Bookmark{nil, bk("https://b.com")},
			wantFresh:      []*bookmark.Bookmark{bk("https://a.com")},
			wantDuplicates: []*bookmark.Bookmark{},
		},
		{
			name:           "both_nil_and_empty",
			bs:             nil,
			existing:       nil,
			wantFresh:      []*bookmark.Bookmark{},
			wantDuplicates: []*bookmark.Bookmark{},
		},
	}

	cmpBookmarks := func(a, b []*bookmark.Bookmark) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i].URL != b[i].URL {
				return false
			}
		}
		return true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotFresh, gotDuplicates := Deduplicate(tt.bs, tt.existing)

			if !cmpBookmarks(gotFresh, tt.wantFresh) {
				t.Errorf("fresh = %v; want %v", gotFresh, tt.wantFresh)
			}
			if !cmpBookmarks(gotDuplicates, tt.wantDuplicates) {
				t.Errorf("duplicates = %v; want %v", gotDuplicates, tt.wantDuplicates)
			}
		})
	}
}
