package bookmark

import (
	"reflect"
	"testing"
)

func TestDeduplicate(t *testing.T) {
	t.Parallel()
	bk := func(url string) *Bookmark {
		return &Bookmark{URL: url}
	}

	tests := []struct {
		name           string
		bs             []*Bookmark
		existing       []*Bookmark
		wantFresh      []*Bookmark
		wantDuplicates []*Bookmark
	}{
		{
			name:           "all_fresh",
			bs:             []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*Bookmark{bk("https://c.com")},
			wantFresh:      []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*Bookmark{},
		},
		{
			name:           "all_duplicates",
			bs:             []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantFresh:      []*Bookmark{},
			wantDuplicates: []*Bookmark{bk("https://a.com"), bk("https://b.com")},
		},
		{
			name:           "mixed_fresh_and_duplicates",
			bs:             []*Bookmark{bk("https://a.com"), bk("https://b.com"), bk("https://c.com")},
			existing:       []*Bookmark{bk("https://b.com")},
			wantFresh:      []*Bookmark{bk("https://a.com"), bk("https://c.com")},
			wantDuplicates: []*Bookmark{bk("https://b.com")},
		},
		{
			name:           "empty_bs",
			bs:             []*Bookmark{},
			existing:       []*Bookmark{bk("https://a.com")},
			wantFresh:      []*Bookmark{},
			wantDuplicates: []*Bookmark{},
		},
		{
			name:           "empty_existing",
			bs:             []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			existing:       []*Bookmark{},
			wantFresh:      []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*Bookmark{},
		},
		{
			name:           "nil_entries_in_bs_are_skipped",
			bs:             []*Bookmark{bk("https://a.com"), nil, bk("https://b.com")},
			existing:       []*Bookmark{},
			wantFresh:      []*Bookmark{bk("https://a.com"), bk("https://b.com")},
			wantDuplicates: []*Bookmark{},
		},
		{
			name:           "nil_entries_in_existing_are_skipped",
			bs:             []*Bookmark{bk("https://a.com")},
			existing:       []*Bookmark{nil, bk("https://b.com")},
			wantFresh:      []*Bookmark{bk("https://a.com")},
			wantDuplicates: []*Bookmark{},
		},
		{
			name:           "both_nil_and_empty",
			bs:             nil,
			existing:       nil,
			wantFresh:      []*Bookmark{},
			wantDuplicates: []*Bookmark{},
		},
	}

	cmpBookmarks := func(a, b []*Bookmark) bool {
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

func TestDifference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []*Bookmark
		b    []*Bookmark
		want []*Bookmark
	}{
		{
			name: "no_difference",
			a: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
			},
			b: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
			},
			want: nil,
		},
		{
			name: "single_missing_in_a",
			a: []*Bookmark{
				{Checksum: "a"},
			},
			b: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
			},
			want: []*Bookmark{
				{Checksum: "b"},
			},
		},
		{
			name: "multiple_missing_in_a",
			a: []*Bookmark{
				{Checksum: "a"},
			},
			b: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
				{Checksum: "c"},
			},
			want: []*Bookmark{
				{Checksum: "b"},
				{Checksum: "c"},
			},
		},
		{
			name: "empty_a_returns_all_b",
			a:    nil,
			b: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
			},
			want: []*Bookmark{
				{Checksum: "a"},
				{Checksum: "b"},
			},
		},
		{
			name: "empty_b_returns_nil",
			a: []*Bookmark{
				{Checksum: "a"},
			},
			b:    nil,
			want: nil,
		},
		{
			name: "nil_entries_are_ignored",
			a: []*Bookmark{
				nil,
				{Checksum: "a"},
			},
			b: []*Bookmark{
				nil,
				{Checksum: "a"},
				{Checksum: "b"},
			},
			want: []*Bookmark{
				{Checksum: "b"},
			},
		},
		{
			name: "single_element_boundary_match",
			a: []*Bookmark{
				{Checksum: "a"},
			},
			b: []*Bookmark{
				{Checksum: "a"},
			},
			want: nil,
		},
		{
			name: "duplicate_checksums_in_b_are_preserved",
			a: []*Bookmark{
				{Checksum: "a"},
			},
			b: []*Bookmark{
				{Checksum: "b"},
				{Checksum: "b"},
			},
			want: []*Bookmark{
				{Checksum: "b"},
				{Checksum: "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Difference(tt.a, tt.b)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("difference() = %#v; want %#v", got, tt.want)
			}
		})
	}
}
