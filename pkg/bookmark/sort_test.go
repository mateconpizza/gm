package bookmark

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"
)

func TestScore(t *testing.T) {
	t.Parallel()

	now := time.Date(2023, 1, 22, 12, 0, 0, 0, time.UTC)
	nowStr := now.Format(time.RFC3339)
	daysAgo := func(d int) string { return now.AddDate(0, 0, -d).Format(time.RFC3339) }

	tests := []struct {
		name string
		b    Bookmark
		want float64
	}{
		// NOTE: IsActive defaults to false, so a never-checked bookmark gets
		// the dead-link penalty, which contradicts the "unchecked is neutral"
		// comment in Score. This case documents the current behaviour.
		{"zero_value_bookmark", Bookmark{}, -wDead},
		{"favorite_active_unchecked", Bookmark{Favorite: true, IsActive: true}, wFav},
		{
			"visits_log_scaled",
			Bookmark{VisitCount: 10, IsActive: true},
			wVisit * math.Log1p(10) / math.Log1p(maxVisitCount),
		},
		{
			"visits_capped_at_max",
			Bookmark{VisitCount: maxVisitCount * 5, IsActive: true},
			wVisit,
		},
		{
			"last_visit_one_half_life_ago",
			Bookmark{LastVisit: daysAgo(21), IsActive: true},
			wRec * 0.5,
		},
		{
			"never_visited_falls_back_to_created_at",
			Bookmark{LastVisit: "", CreatedAt: nowStr, IsActive: true},
			wNew,
		},
		{
			"status_299_is_alive",
			Bookmark{HTTPStatusCode: 299, IsActive: true},
			wAlive,
		},
		{
			"status_400_is_dead",
			Bookmark{HTTPStatusCode: 400, IsActive: true},
			-wDead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Score(&tt.b, now)

			const eps = 1e-12
			if diff := got - tt.want; diff < -eps || diff > eps {
				t.Fatalf("Score(%+v) = %v; want %v", tt.b, got, tt.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sort    string
		in      []Bookmark
		want    []int // expected IDs, in order
		wantErr error
	}{
		{
			"empty_option_keeps_order", "",
			[]Bookmark{{ID: 3}, {ID: 1}, {ID: 2}},
			[]int{3, 1, 2},
			nil,
		},
		{
			"empty_slice", "newest",
			nil,
			nil, nil,
		},
		{
			"invalid_option", "nope",
			[]Bookmark{{ID: 1}},
			nil, ErrInvalidOption,
		},
		{
			"favorite_first_stable", "favorite",
			[]Bookmark{{ID: 1}, {ID: 2, Favorite: true}, {ID: 3}, {ID: 4, Favorite: true}},
			[]int{2, 4, 1, 3},
			nil,
		},
		{
			"alias_fav", "fav",
			[]Bookmark{{ID: 1}, {ID: 2, Favorite: true}},
			[]int{2, 1},
			nil,
		},
		{
			"alias_normalized_input", "  Last_Visit ",
			[]Bookmark{{ID: 1, LastVisit: "2023-01-01T00:00:00Z"}, {ID: 2, LastVisit: "2023-02-01T00:00:00Z"}},
			[]int{2, 1},
			nil,
		},
		{
			"newest_ties_by_id_missing_last", "newest",
			[]Bookmark{
				{ID: 1, CreatedAt: "2023-01-01T00:00:00Z"},
				{ID: 2, CreatedAt: "2023-03-01T00:00:00Z"},
				{ID: 3, CreatedAt: "2023-03-01T00:00:00Z"},
				{ID: 4},
			},
			[]int{3, 2, 1, 4},
			nil,
		},
		{
			"oldest_missing_first", "oldest",
			[]Bookmark{
				{ID: 1, CreatedAt: "2023-01-01T00:00:00Z"},
				{ID: 2, CreatedAt: "2023-03-01T00:00:00Z"},
				{ID: 3, CreatedAt: "2023-03-01T00:00:00Z"},
				{ID: 4},
			},
			[]int{4, 1, 2, 3},
			nil,
		},
		{
			"popular_ties_by_last_visit", "popular",
			[]Bookmark{
				{ID: 1, VisitCount: 5},
				{ID: 2, VisitCount: 10, LastVisit: "2023-01-01T00:00:00Z"},
				{ID: 3, VisitCount: 10, LastVisit: "2023-02-01T00:00:00Z"},
				{ID: 4},
			},
			[]int{3, 2, 1, 4},
			nil,
		},
		{
			"visited_missing_last_ties_by_visits", "visited",
			[]Bookmark{
				{ID: 1, LastVisit: "2023-01-01T00:00:00Z", VisitCount: 1},
				{ID: 2},
				{ID: 3, LastVisit: "2023-02-01T00:00:00Z"},
				{ID: 4, LastVisit: "2023-01-01T00:00:00Z", VisitCount: 9},
			},
			[]int{3, 4, 1, 2},
			nil,
		},
		{
			"inactive_first_highest_status", "inactive",
			[]Bookmark{
				{ID: 1, IsActive: true, HTTPStatusCode: 200},
				{ID: 2, HTTPStatusCode: 404},
				{ID: 3, HTTPStatusCode: 500},
				{ID: 4, IsActive: true},
			},
			[]int{3, 2, 1, 4},
			nil,
		},
		{
			"reverse_by_id", "reverse",
			[]Bookmark{{ID: 1}, {ID: 2}, {ID: 3}},
			[]int{3, 2, 1},
			nil,
		},
		{
			"domain_no_domain_first_title_tiebreak", "domain",
			[]Bookmark{
				{ID: 1, URL: "https://www.b.com", Title: "x"},
				{ID: 2, URL: "https://a.com", Title: "Zed"},
				{ID: 3, URL: "https://a.com/other", Title: "alpha"},
				{ID: 4, URL: ""},
			},
			[]int{4, 3, 2, 1},
			nil,
		},
		{
			"unvisited_ties_by_newest", "unvisited",
			[]Bookmark{
				{ID: 1, VisitCount: 3},
				{ID: 2, CreatedAt: "2023-01-01T00:00:00Z"},
				{ID: 3, CreatedAt: "2023-02-01T00:00:00Z"},
			},
			[]int{3, 2, 1},
			nil,
		},
		{
			"unchecked_never_checked_first", "unchecked",
			[]Bookmark{
				{ID: 1, LastStatusChecked: "2023-02-01T00:00:00Z"},
				{ID: 2},
				{ID: 3, LastStatusChecked: "2023-01-01T00:00:00Z"},
			},
			[]int{2, 3, 1},
			nil,
		},
		{
			"stale_never_visited_first", "stale",
			[]Bookmark{
				{ID: 1, LastVisit: "2023-02-01T00:00:00Z"},
				{ID: 2, CreatedAt: "2023-01-01T00:00:00Z"},
				{ID: 3, LastVisit: "2023-01-01T00:00:00Z"},
				{ID: 4},
			},
			[]int{4, 2, 3, 1},
			nil,
		},
		{
			"updated_missing_last", "updated",
			[]Bookmark{
				{ID: 1, UpdatedAt: "2023-01-01T00:00:00Z"},
				{ID: 2, UpdatedAt: "2023-02-01T00:00:00Z"},
				{ID: 3},
			},
			[]int{2, 1, 3},
			nil,
		},
		{
			// no dates, so the result doesn't depend on time.Now()
			"smart_alive_before_dead", "smart",
			[]Bookmark{
				{ID: 2, HTTPStatusCode: 404},
				{ID: 1, IsActive: true, HTTPStatusCode: 200},
			},
			[]int{1, 2},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bs := make([]*Bookmark, len(tt.in))
			for i := range tt.in {
				bs[i] = &tt.in[i]
			}

			got, err := Sort(tt.sort, bs)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Sort(%q) expected error %v, got nil", tt.sort, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Sort(%q) error = %v; want errors.Is match for %v", tt.sort, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Sort(%q) unexpected error: %v", tt.sort, err)
			}

			ids := make([]int, len(got))
			for i, b := range got {
				ids[i] = b.ID
			}

			if !slices.Equal(ids, tt.want) {
				t.Fatalf("Sort(%q) order = %v; want %v", tt.sort, ids, tt.want)
			}
		})
	}
}

func TestSmartFavorites(t *testing.T) {
	t.Parallel()

	mixed := []Bookmark{
		{ID: 1, URL: "https://a.com", Favorite: true, VisitCount: 1},
		{ID: 2, URL: "https://b.com", VisitCount: 100}, // not a favorite
		{ID: 3, URL: "https://c.com", Favorite: true, VisitCount: 50},
		{ID: 4, URL: "https://d.com", Favorite: true, VisitCount: 10},
	}

	tests := []struct {
		name  string
		in    []Bookmark
		limit int
		want  []int // expected IDs, in order
	}{
		{"nil_input", nil, 5, nil},
		{"no_favorites", []Bookmark{{ID: 1}, {ID: 2}}, 5, nil},
		{"only_favorites_sorted_by_score", mixed, 5, []int{3, 4, 1}},
		{"limit_zero_returns_all", mixed, 0, []int{3, 4, 1}},
		{"limit_negative_returns_all", mixed, -1, []int{3, 4, 1}},
		{"limit_truncates", mixed, 2, []int{3, 4}},
		{"limit_one", mixed, 1, []int{3}},
		{"limit_equal_to_count", mixed, 3, []int{3, 4, 1}},
		{
			"equal_scores_tie_break_by_url",
			[]Bookmark{
				{ID: 1, URL: "https://b.com", Favorite: true},
				{ID: 2, URL: "https://a.com", Favorite: true},
			},
			0,
			[]int{2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bs := make([]*Bookmark, len(tt.in))
			inIDs := make([]int, len(tt.in))
			for i := range tt.in {
				bs[i] = &tt.in[i]
				inIDs[i] = tt.in[i].ID
			}

			got := SmartFavorites(bs, tt.limit)

			ids := make([]int, len(got))
			for i, b := range got {
				ids[i] = b.ID
			}

			if !slices.Equal(ids, tt.want) {
				t.Fatalf("SmartFavorites(limit=%d) order = %v; want %v", tt.limit, ids, tt.want)
			}

			// the input slice must keep its original order
			afterIDs := make([]int, len(bs))
			for i, b := range bs {
				afterIDs[i] = b.ID
			}
			if !slices.Equal(afterIDs, inIDs) {
				t.Fatalf("SmartFavorites modified the input order: got %v; want %v", afterIDs, inIDs)
			}
		})
	}
}
