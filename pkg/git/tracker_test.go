package git

import (
	"errors"
	"slices"
	"testing"
)

func TestTracker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repos   []string
		action  func(tr *Tracker) error
		want    []string
		wantErr error
	}{
		{
			name:  "track_single_repo",
			repos: nil,
			action: func(tr *Tracker) error {
				return tr.track("main")
			},
			want: []string{"main"},
		},
		{
			name:  "track_multiple_repos",
			repos: []string{"main"},
			action: func(tr *Tracker) error {
				return tr.track("other", "third")
			},
			want: []string{"main", "other", "third"},
		},
		{
			name:    "track_empty_names",
			repos:   nil,
			action:  func(tr *Tracker) error { return tr.track() },
			want:    nil,
			wantErr: ErrGitRepoNameEmpty,
		},
		{
			name:  "untrack_existing_repo",
			repos: []string{"main", "other", "third"},
			action: func(tr *Tracker) error {
				return tr.untrack("other")
			},
			want: []string{"main", "third"},
		},
		{
			name:  "untrack_missing_repo",
			repos: []string{"main", "other"},
			action: func(tr *Tracker) error {
				return tr.untrack("missing")
			},
			want: []string{"main", "other"},
		},
		{
			name:    "untrack_empty_name",
			repos:   []string{"main"},
			action:  func(tr *Tracker) error { return tr.untrack("") },
			want:    []string{"main"},
			wantErr: ErrGitRepoNameEmpty,
		},
		{
			name:  "write_compacts_adjacent_duplicates",
			repos: []string{"main", "main", "other", "other", "third"},
			action: func(tr *Tracker) error {
				return tr.write()
			},
			want: []string{"main", "other", "third"},
		},
		{
			name:  "load_persisted_repos",
			repos: nil,
			action: func(tr *Tracker) error {
				if err := writeFile(tr.filename, &[]string{"main", "other"}); err != nil {
					return err
				}
				return tr.load()
			},
			want: []string{"main", "other"},
		},
		{
			name:  "reset_repos",
			repos: []string{"main", "main", "other", "other", "third"},
			action: func(tr *Tracker) error {
				tr.reset()
				return nil
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracker := newTracker(t.TempDir())
			tracker.repos = tt.repos

			err := tt.action(tracker)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantErr != nil {
				return
			}

			if !slices.Equal(tracker.list(), tt.want) {
				t.Fatalf("repos = %v; want %v", tracker.list(), tt.want)
			}
		})
	}
}
