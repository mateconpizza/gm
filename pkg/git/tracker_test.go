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
				if err := tr.track("main", "other"); err != nil {
					return err
				}
				if err := tr.write(); err != nil {
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
			tracker.file.value = tt.repos

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

func TestTracker_untrack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		initialState []string
		untrackName  string
		wantState    []string
		wantErr      error
	}{
		{
			name:         "normal_untrack",
			initialState: []string{"backend", "frontend", "infra"},
			untrackName:  "frontend",
			wantState:    []string{"backend", "infra"},
			wantErr:      nil,
		},
		{
			name:         "empty_repo_name",
			initialState: []string{"backend"},
			untrackName:  "",
			wantState:    []string{"backend"},
			wantErr:      ErrGitRepoNameEmpty,
		},
		{
			name:         "repo_not_found",
			initialState: []string{"backend", "frontend"},
			untrackName:  "mobile",
			wantState:    []string{"backend", "frontend"},
			wantErr:      nil,
		},
		{
			name:         "multiple_occurrences",
			initialState: []string{"backend", "frontend", "backend"},
			untrackName:  "backend",
			wantState:    []string{"frontend"},
			wantErr:      nil,
		},
		{
			name:         "only_element",
			initialState: []string{"backend"},
			untrackName:  "backend",
			wantState:    []string{},
			wantErr:      nil,
		},
		{
			name:         "empty_tracker",
			initialState: []string{},
			untrackName:  "backend",
			wantState:    []string{},
			wantErr:      nil,
		},
		{
			name:         "exact_match_only",
			initialState: []string{"backend-api"},
			untrackName:  "backend",
			wantState:    []string{"backend-api"},
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Initialize Tracker with the desired state.
			// Recreate the slice to avoid modifying the test definition accidentally.
			initial := make([]string, len(tt.initialState))
			copy(initial, tt.initialState)

			tr := &Tracker{
				file: &JSONFile[[]string]{
					value: initial,
				},
			}

			err := tr.untrack(tt.untrackName)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Tracker.untrack(%q) expected error %v, got nil", tt.untrackName, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Tracker.untrack(%q) expected error %v, got %v", tt.untrackName, tt.wantErr, err)
				}
				// Verify state was not unintentionally modified during error
				if !slices.Equal(tr.file.value, tt.wantState) {
					t.Errorf("Tracker.untrack(%q) state = %v; want %v (after error)", tt.untrackName, tr.file.value, tt.wantState)
				}
				return
			}

			if err != nil {
				t.Fatalf("Tracker.untrack(%q) unexpected error: %v", tt.untrackName, err)
			}

			if !slices.Equal(tr.file.value, tt.wantState) {
				t.Errorf("Tracker.untrack(%q) state = %v; want %v", tt.untrackName, tr.file.value, tt.wantState)
			}
		})
	}
}

func TestTracker_write(t *testing.T) {
	t.Parallel()

	errWriteFailed := errors.New("write failed")

	tests := []struct {
		name     string
		initial  []string // value of t.file.value before Write is called
		writeErr error    // error returned by the fake writeFunc, if any
		want     []string // expected sorted+deduped slice passed to writeFunc
		wantErr  error
	}{
		{
			name:    "normal_unsorted_with_dupes",
			initial: []string{"work", "main", "work", "org"},
			want:    []string{"main", "org", "work"},
			wantErr: nil,
		},
		{
			name:    "already_sorted_no_dupes",
			initial: []string{"a", "b", "c"},
			want:    []string{"a", "b", "c"},
			wantErr: nil,
		},
		{
			name:    "empty_slice",
			initial: []string{},
			want:    []string{},
			wantErr: nil,
		},
		{
			name:    "nil_slice",
			initial: nil,
			want:    nil,
			wantErr: nil,
		},
		{
			name:    "single_element",
			initial: []string{"main"},
			want:    []string{"main"},
			wantErr: nil,
		},
		{
			name:    "all_duplicates",
			initial: []string{"x", "x", "x"},
			want:    []string{"x"},
			wantErr: nil,
		},
		{
			name:    "non_adjacent_dupes_after_sort",
			initial: []string{"c", "a", "b", "a"},
			want:    []string{"a", "b", "c"},
			wantErr: nil,
		},
		{
			name:     "write_func_error_propagated",
			initial:  []string{"main", "work"},
			writeErr: errWriteFailed,
			want:     []string{"main", "work"},
			wantErr:  errWriteFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			tr := &Tracker{
				file: &JSONFile[[]string]{
					path:  "unused.json",
					value: tt.initial,
					writeFunc: func(_ string, v *[]string) error {
						captured = *v
						return tt.writeErr
					},
				},
			}

			err := tr.write()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Write() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Write() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Write() unexpected error: %v", err)
			}

			if !slices.Equal(captured, tt.want) {
				t.Fatalf("Write() passed %v to writeFunc; want %v", captured, tt.want)
			}

			if !slices.Equal(tr.file.value, tt.want) {
				t.Fatalf("Write() left file.value = %v; want %v", tr.file.value, tt.want)
			}
		})
	}
}

func TestTracker_Load(t *testing.T) {
	t.Parallel()

	errMockRead := errors.New("mock read error")

	tests := []struct {
		name         string
		mockExists   bool
		mockReadErr  error
		mockReadData []string
		wantState    []string
		wantErr      error
	}{
		{
			name:         "file_exists_and_reads_successfully",
			mockExists:   true,
			mockReadErr:  nil,
			mockReadData: []string{"backend", "frontend"},
			wantState:    []string{"backend", "frontend"},
			wantErr:      nil,
		},
		{
			name:         "file_does_not_exist",
			mockExists:   false,
			mockReadErr:  nil,
			mockReadData: nil,
			wantState:    nil, // Should not modify state
			wantErr:      nil,
		},
		{
			name:         "file_exists_but_read_fails",
			mockExists:   true,
			mockReadErr:  errMockRead,
			mockReadData: nil,
			wantState:    nil,
			wantErr:      errMockRead,
		},
		{
			name:         "file_exists_empty_content",
			mockExists:   true,
			mockReadErr:  nil,
			mockReadData: []string{},
			wantState:    []string{},
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := &Tracker{
				file: &JSONFile[[]string]{
					path: "/mock/tracker.json",
					existsFunc: func(path string) bool {
						return tt.mockExists
					},
					readFunc: func(path string, value *[]string) error {
						if tt.mockReadErr == nil && value != nil {
							*value = tt.mockReadData
						}
						return tt.mockReadErr
					},
				},
			}

			err := tr.load()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Tracker.Load() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Tracker.Load() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Tracker.Load() unexpected error: %v", err)
			}

			if tt.mockExists && !slices.Equal(tr.file.value, tt.wantState) {
				t.Errorf("Tracker.Load() state = %v; want %v", tr.file.value, tt.wantState)
			}
		})
	}
}
