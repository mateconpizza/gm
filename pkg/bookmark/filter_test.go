package bookmark

import (
	"errors"
	"reflect"
	"slices"
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

func TestFilterBy(t *testing.T) {
	t.Parallel()

	all := []Bookmark{
		// 1: favorite, active, visited, tagged, checked
		{ID: 1, Favorite: true, IsActive: true, VisitCount: 5, Tags: "go", LastStatusChecked: "2023-01-01T00:00:00Z"},
		// 2: inactive, checked, unvisited, untagged
		{ID: 2, IsActive: false, LastStatusChecked: "2023-01-01T00:00:00Z"},
		// 3: favorite, inactive, never checked, unvisited, untagged
		{ID: 3, Favorite: true},
		// 4: active, visited, tagged, checked, with notes, archived
		{
			ID: 4, IsActive: true, VisitCount: 2, Tags: "a,b", Notes: "read later",
			ArchiveURL: "https://web.archive.org/web/1/https://example.com", LastStatusChecked: "2023-01-01T00:00:00Z",
		},
	}

	tests := []struct {
		name    string
		filter  string
		nilIn   bool
		want    []int // expected IDs, in order
		wantErr error
	}{
		{"empty_filter_returns_all", "", false, []int{1, 2, 3, 4}, nil},
		{"only_separators_returns_all", " , ,", false, []int{1, 2, 3, 4}, nil},
		{"single_filter", "favorite", false, []int{1, 3}, nil},
		{"alias", "fav", false, []int{1, 3}, nil},
		{"alias_unvisited", "never", false, []int{2, 3}, nil},
		{"uppercase_and_spaces", "  FAVORITE ", false, []int{1, 3}, nil},
		{"negated", "!favorite", false, []int{2, 4}, nil},
		{"negated_with_space_after_bang", "! favorite", false, []int{2, 4}, nil},
		{"combined_and", "favorite,!unvisited", false, []int{1}, nil},
		{"combined_inactive_but_checked", "inactive,!unchecked", false, []int{2}, nil},
		{"combined_with_spaces", "favorite , inactive", false, []int{3}, nil},
		{"duplicate_filter", "favorite,fav", false, []int{1, 3}, nil},
		{"filters_exclude_each_other", "archived,untagged", false, nil, nil},
		{"nil_input_with_filter", "favorite", true, nil, nil},
		{"unknown_filter", "nope", false, nil, ErrInvalidFilter},
		{"unknown_among_valid", "favorite,nope", false, nil, ErrInvalidFilter},
		{"bang_only", "!", false, nil, ErrInvalidFilter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var bs []*Bookmark
			var inIDs []int
			if !tt.nilIn {
				bs = make([]*Bookmark, len(all))
				for i := range all {
					b := all[i] // copy, so parallel subtests don't share state
					bs[i] = &b
					inIDs = append(inIDs, b.ID)
				}
			}

			got, err := FilterBy(tt.filter, bs)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("FilterBy(%q) expected error %v, got nil", tt.filter, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("FilterBy(%q) error = %v; want errors.Is match for %v", tt.filter, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FilterBy(%q) unexpected error: %v", tt.filter, err)
			}

			ids := make([]int, len(got))
			for i, b := range got {
				ids[i] = b.ID
			}
			if !slices.Equal(ids, tt.want) {
				t.Fatalf("FilterBy(%q) = %v; want %v", tt.filter, ids, tt.want)
			}

			// the input slice must keep its original order and length
			afterIDs := make([]int, len(bs))
			for i, b := range bs {
				afterIDs[i] = b.ID
			}
			if !slices.Equal(afterIDs, inIDs) {
				t.Fatalf("FilterBy(%q) modified the input: got %v; want %v", tt.filter, afterIDs, inIDs)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        []int // IDs
		predicate func(b *Bookmark) bool
		want      []int
	}{
		{"keeps_matching_in_order", []int{1, 2, 3, 4}, func(b *Bookmark) bool { return b.ID%2 == 0 }, []int{2, 4}},
		{"nil_input", nil, func(b *Bookmark) bool { return true }, nil},
		{"empty_input", []int{}, func(b *Bookmark) bool { return true }, nil},
		{"none_match", []int{1, 2}, func(b *Bookmark) bool { return false }, nil},
		{"all_match", []int{1, 2}, func(b *Bookmark) bool { return true }, []int{1, 2}},
		{"single_element_match", []int{7}, func(b *Bookmark) bool { return b.ID == 7 }, []int{7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var bs []*Bookmark
			for _, id := range tt.in {
				bs = append(bs, &Bookmark{ID: id})
			}

			got := Filter(bs, tt.predicate)

			ids := make([]int, len(got))
			for i, b := range got {
				ids[i] = b.ID
			}
			if !slices.Equal(ids, tt.want) {
				t.Fatalf("Filter(%v) = %v; want %v", tt.in, ids, tt.want)
			}
		})
	}
}
