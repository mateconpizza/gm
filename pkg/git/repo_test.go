package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestRepo_Add(t *testing.T) {
	t.Parallel()

	errMockWriter := errors.New("mock writer error")

	b1 := &bookmark.Bookmark{}
	b2 := &bookmark.Bookmark{}
	b3 := &bookmark.Bookmark{}

	tests := []struct {
		name          string
		initBookmarks []*bookmark.Bookmark
		writer        WriterFunc
		input         []*bookmark.Bookmark
		wantBookmarks []*bookmark.Bookmark
		wantErr       error
	}{
		{
			name:          "normal_add",
			initBookmarks: nil,
			writer: func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return nil
			},
			input:         []*bookmark.Bookmark{b1, b2},
			wantBookmarks: []*bookmark.Bookmark{b1, b2},
			wantErr:       nil,
		},
		{
			name:          "append_to_existing",
			initBookmarks: []*bookmark.Bookmark{b1},
			writer: func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return nil
			},
			input:         []*bookmark.Bookmark{b2, b3},
			wantBookmarks: []*bookmark.Bookmark{b1, b2, b3},
			wantErr:       nil,
		},
		{
			name:          "nil_writer",
			initBookmarks: nil,
			writer:        nil,
			input:         []*bookmark.Bookmark{b1},
			wantBookmarks: nil,
			wantErr:       ErrNoFunctionFound,
		},
		{
			name:          "writer_error",
			initBookmarks: []*bookmark.Bookmark{b1},
			writer: func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return errMockWriter
			},
			input:         []*bookmark.Bookmark{b2},
			wantBookmarks: []*bookmark.Bookmark{b1},
			wantErr:       errMockWriter,
		},
		{
			name:          "empty_input_slice",
			initBookmarks: []*bookmark.Bookmark{b1},
			writer: func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return nil
			},
			input:         []*bookmark.Bookmark{},
			wantBookmarks: []*bookmark.Bookmark{b1},
			wantErr:       nil,
		},
		{
			name:          "nil_input_slice",
			initBookmarks: []*bookmark.Bookmark{b1},
			writer: func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return nil
			},
			input:         nil,
			wantBookmarks: []*bookmark.Bookmark{b1},
			wantErr:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []RepoOptFunc
			if tt.writer != nil {
				opts = append(opts, WithRepoWriter(tt.writer))
			}

			r := NewRepo(tt.name, t.TempDir(), opts...)
			r.bookmarks = tt.initBookmarks

			err := r.Add(t.Context(), tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Add() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Add() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Add() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(r.bookmarks, tt.wantBookmarks) {
				t.Fatalf("Add() bookmarks = %v; want %v", r.bookmarks, tt.wantBookmarks)
			}
		})
	}
}

func TestRepo_Misc(t *testing.T) {
	t.Parallel()

	repos := make([]string, 0, 5)
	for i := range 5 {
		repos = append(repos, fmt.Sprintf("repo-%d", i+1))
	}

	for _, repoName := range repos {
		t.Run(repoName, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			tempRoot := filepath.Dir(tempDir)
			r := NewRepo(repoName, tempDir)

			if r.Name() != repoName {
				t.Fatalf("Name() wrong name: want %q, got %q", repoName, r.Name())
			}
			if r.Fullpath() != tempDir {
				t.Fatalf("Fullpath() wrong root path: want %q, got %q", tempDir, r.Fullpath())
			}
			if r.Root() != tempRoot {
				t.Fatalf("Root() wrong root path: want %q, got %q", tempRoot, r.Root())
			}
		})
	}
}

func TestRepo_RmManyTwo(t *testing.T) {
	t.Parallel()

	b1 := &bookmark.Bookmark{ID: 1, URL: "https://a.com"}
	b2 := &bookmark.Bookmark{ID: 2, URL: "https://b.com"}
	b3 := &bookmark.Bookmark{ID: 3, URL: "https://c.com"}

	errCleanupFail := errors.New("cleanup failed")
	errDiskFull := errors.New("disk full")

	tests := []struct {
		name          string
		initial       []*bookmark.Bookmark
		toRemove      []*bookmark.Bookmark
		noRemover     bool
		removerErr    error
		postRmErr     error
		wantErr       error
		wantRemaining []*bookmark.Bookmark
	}{
		{
			name:          "removes_matching_bookmark",
			initial:       []*bookmark.Bookmark{b1, b2, b3},
			toRemove:      []*bookmark.Bookmark{b2},
			wantRemaining: []*bookmark.Bookmark{b1, b3},
		},
		{
			name:          "empty_removal_list_no_op",
			initial:       []*bookmark.Bookmark{b1, b2},
			toRemove:      []*bookmark.Bookmark{},
			wantRemaining: []*bookmark.Bookmark{b1, b2},
		},
		{
			name:          "removes_all_bookmarks",
			initial:       []*bookmark.Bookmark{b1, b2},
			toRemove:      []*bookmark.Bookmark{b1, b2},
			wantRemaining: []*bookmark.Bookmark{},
		},
		{
			name:          "url_not_present_is_noop",
			initial:       []*bookmark.Bookmark{b1},
			toRemove:      []*bookmark.Bookmark{{ID: 99, URL: "https://missing.com"}},
			wantRemaining: []*bookmark.Bookmark{b1},
		},
		{
			name:          "duplicate_urls_in_removal_list",
			initial:       []*bookmark.Bookmark{b1, b2, b3},
			toRemove:      []*bookmark.Bookmark{b2, {ID: 20, URL: b2.URL}},
			wantRemaining: []*bookmark.Bookmark{b1, b3},
		},
		{
			name:      "no_remover_configured",
			initial:   []*bookmark.Bookmark{b1},
			toRemove:  []*bookmark.Bookmark{b1},
			noRemover: true,
			wantErr:   ErrNoFunctionFound,
		},
		{
			name:       "remover_fails",
			initial:    []*bookmark.Bookmark{b1},
			toRemove:   []*bookmark.Bookmark{b1},
			removerErr: errDiskFull,
			wantErr:    errDiskFull,
		},
		{
			name:      "post_removal_fails",
			initial:   []*bookmark.Bookmark{b1, b2},
			toRemove:  []*bookmark.Bookmark{b1},
			postRmErr: errCleanupFail,
			wantErr:   errCleanupFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []RepoOptFunc

			if !tt.noRemover {
				opts = append(opts, WithRepoRemover(func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
					return tt.removerErr
				}))
			}

			gr := NewRepo(tt.name, t.TempDir(), opts...)
			gr.bookmarks = append([]*bookmark.Bookmark(nil), tt.initial...)

			postRm := func(path string) error { return tt.postRmErr }

			err := gr.RmMany(t.Context(), tt.toRemove, postRm)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("RmMany() error = %v, want %v", err, tt.wantErr)
				}
				return
			case err != nil:
				t.Fatalf("RmMany() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(gr.bookmarks, tt.wantRemaining) {
				t.Errorf("bookmarks = %v, want %v", gr.bookmarks, tt.wantRemaining)
			}
		})
	}
}

func TestRepo_Rm(t *testing.T) {
	t.Parallel()

	b1 := &bookmark.Bookmark{ID: 1, URL: "https://a.com"}
	b2 := &bookmark.Bookmark{ID: 2, URL: "https://b.com"}
	b3 := &bookmark.Bookmark{ID: 3, URL: "https://c.com"}

	errRemover := errors.New("disk full")
	errPostRm := errors.New("cleanup failed")

	tests := []struct {
		name          string
		initial       []*bookmark.Bookmark
		target        *bookmark.Bookmark
		noRemover     bool
		removerErr    error
		postRmErr     error
		want          error
		wantRemaining []*bookmark.Bookmark
	}{
		{
			name:          "removes_matching_bookmark",
			initial:       []*bookmark.Bookmark{b1, b2, b3},
			target:        b2,
			wantRemaining: []*bookmark.Bookmark{b1, b3},
		},
		{
			name:          "removes_only_matching_id",
			initial:       []*bookmark.Bookmark{b1},
			target:        b1,
			wantRemaining: []*bookmark.Bookmark{},
		},
		{
			name:          "id_not_present_is_noop",
			initial:       []*bookmark.Bookmark{b1, b2},
			target:        &bookmark.Bookmark{ID: 99, URL: "https://missing.com"},
			wantRemaining: []*bookmark.Bookmark{b1, b2},
		},
		{
			name:          "empty_repo_is_noop",
			initial:       []*bookmark.Bookmark{},
			target:        b1,
			wantRemaining: []*bookmark.Bookmark{},
		},
		{
			name:      "no_remover_configured",
			initial:   []*bookmark.Bookmark{b1},
			target:    b1,
			noRemover: true,
			want:      ErrNoFunctionFound,
		},
		{
			name:       "remover_fails",
			initial:    []*bookmark.Bookmark{b1},
			target:     b1,
			removerErr: errRemover,
			want:       errRemover,
		},
		{
			name:      "post_removal_fails",
			initial:   []*bookmark.Bookmark{b1, b2},
			target:    b1,
			postRmErr: errPostRm,
			want:      errPostRm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []RepoOptFunc
			if !tt.noRemover {
				opts = append(opts, WithRepoRemover(func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
					return tt.removerErr
				}))
			}

			gr := NewRepo("myrepo", t.TempDir(), opts...)
			gr.bookmarks = append([]*bookmark.Bookmark(nil), tt.initial...)

			postRm := func(path string) error { return tt.postRmErr }

			err := gr.Rm(t.Context(), tt.target, postRm)

			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("Rm() error = %v, want %v", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("Rm() unexpected error: %v", err)
			}

			if !slices.EqualFunc(gr.bookmarks, tt.wantRemaining, func(a, b *bookmark.Bookmark) bool {
				return reflect.DeepEqual(a, b)
			}) {
				t.Errorf("bookmarks = %v, want %v", gr.bookmarks, tt.wantRemaining)
			}
		})
	}
}

func TestRepo_Read(t *testing.T) {
	t.Parallel()

	b1 := &bookmark.Bookmark{ID: 1, URL: "https://a.com"}
	b2 := &bookmark.Bookmark{ID: 2, URL: "https://b.com"}

	errReader := errors.New("read failed")

	tests := []struct {
		name          string
		noReader      bool
		summaryFile   []byte // raw bytes written to gr.summaryFile before Read(); nil = no file
		readerBS      []*bookmark.Bookmark
		readerErr     error
		wantErr       error
		wantTotal     int // total the reader should have been called with
		wantBookmarks []*bookmark.Bookmark
	}{
		{
			name:          "reads_and_sets_bookmarks",
			readerBS:      []*bookmark.Bookmark{b1, b2},
			wantTotal:     0, // no summary file present -> Count() returns 0
			wantBookmarks: []*bookmark.Bookmark{b1, b2},
		},
		{
			name:          "reader_returns_empty",
			readerBS:      []*bookmark.Bookmark{},
			wantTotal:     0,
			wantBookmarks: []*bookmark.Bookmark{},
		},
		{
			name:          "reader_returns_nil_clears_bookmarks",
			readerBS:      nil,
			wantTotal:     0,
			wantBookmarks: nil,
		},
		{
			name:     "no_reader_configured",
			noReader: true,
			wantErr:  ErrNoFunctionFound,
		},
		{
			name:      "reader_fails",
			readerErr: errReader,
			wantErr:   errReader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			var gotTotal int
			var readerCalled bool

			var opts []RepoOptFunc
			if !tt.noReader {
				opts = append(opts, WithRepoReader(func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
					readerCalled = true
					gotTotal = total
					return tt.readerBS, tt.readerErr
				}))
			}

			gr := NewRepo(tt.name, tempDir, opts...)

			if tt.summaryFile != nil {
				if err := os.WriteFile(gr.summaryFile, tt.summaryFile, 0o644); err != nil {
					t.Fatalf("setup: writing summary file: %v", err)
				}
			}

			err := gr.Read(t.Context())

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Read() error = %v, want %v", err, tt.wantErr)
				}
				return
			case tt.summaryFile != nil:
				if err == nil {
					t.Fatalf("Read() error = nil, want an error from Count()")
				}
				if readerCalled {
					t.Error("reader was called despite Count() failing")
				}
				return
			case err != nil:
				t.Fatalf("Read() unexpected error: %v", err)
			}

			if !readerCalled {
				t.Fatal("reader was never called")
			}
			if gotTotal != tt.wantTotal {
				t.Errorf("reader total = %d, want %d", gotTotal, tt.wantTotal)
			}
			if !slices.EqualFunc(gr.bookmarks, tt.wantBookmarks, func(a, b *bookmark.Bookmark) bool {
				return reflect.DeepEqual(a, b)
			}) {
				t.Errorf("bookmarks = %v, want %v", gr.bookmarks, tt.wantBookmarks)
			}
		})
	}
}

func TestRepo_Count(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
		want        int
		wantErr     error
	}{
		{
			name:      "no_summary_file_returns_zero",
			setupFile: false,
			want:      0,
			wantErr:   nil,
		},
		{
			name:        "valid_summary_with_bookmarks",
			setupFile:   true,
			fileContent: `{"stats":{"bookmarks":42}}`,
			want:        42,
			wantErr:     nil,
		},
		{
			name:        "valid_summary_nil_stats",
			setupFile:   true,
			fileContent: `{}`,
			want:        0,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			summaryPath := filepath.Join(tempDir, "summary.json")

			if tt.setupFile {
				if err := os.WriteFile(summaryPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("failed to setup summary file: %v", err)
				}
			}

			r := &Repo{
				summaryFile: summaryPath,
			}

			got, err := r.Count()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Count() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Count() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("Count() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestRepo_Summary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
		wantStats   *RepoStats
		wantErr     bool
	}{
		{
			name:      "no_summary_file",
			wantStats: nil,
		},
		{
			name:        "valid_summary",
			setupFile:   true,
			fileContent: `{"stats":{"bookmarks":42}}`,
			wantStats:   &RepoStats{Bookmarks: 42},
		},
		{
			name:        "nil_stats",
			setupFile:   true,
			fileContent: `{}`,
			wantStats:   nil,
		},
		{
			name:        "invalid_json",
			setupFile:   true,
			fileContent: `[invalid-data}`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			summaryPath := filepath.Join(tempDir, "summary.json")

			if tt.setupFile {
				if err := os.WriteFile(summaryPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("failed to setup summary file: %v", err)
				}
			}

			r := &Repo{
				summaryFile: summaryPath,
			}

			got, err := r.Summary()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Summary() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Summary() unexpected error: %v", err)
			}

			if got == nil {
				t.Fatalf("Summary() returned nil despite no error")
			}
		})
	}
}

type mockRepoDB struct {
	statsErr error
}

func (m *mockRepoDB) Stats(ctx context.Context, dest any) error {
	if m.statsErr != nil {
		return m.statsErr
	}
	if stats, ok := dest.(*RepoStats); ok {
		stats.Bookmarks = 10
		stats.Tags = 5
	}
	return nil
}

func TestRepo_Stats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		repoName    string
		setupFile   bool
		fileContent string
		wantStats   *RepoStats
		wantErr     bool
	}{
		{
			name:      "no_summary_file",
			repoName:  "my-repo",
			setupFile: false,
			wantStats: &RepoStats{},
			wantErr:   false,
		},
		{
			name:        "valid_summary_file",
			repoName:    "my-repo",
			setupFile:   true,
			fileContent: `{"stats":{"bookmarks":15,"tags":3}}`,
			wantStats:   &RepoStats{Name: "my-repo", Bookmarks: 15, Tags: 3},
			wantErr:     false,
		},
		{
			name:        "invalid_summary_file_format",
			repoName:    "my-repo",
			setupFile:   true,
			fileContent: `invalid-json`,
			wantStats:   nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			summaryPath := filepath.Join(tempDir, "summary.json")

			if tt.setupFile {
				if err := os.WriteFile(summaryPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("failed to setup file: %v", err)
				}
			}

			r := &Repo{
				name:        tt.repoName,
				summaryFile: summaryPath,
			}

			got, err := r.Stats()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Stats() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Stats() unexpected error: %v", err)
			}

			if got.Name != tt.wantStats.Name || got.Bookmarks != tt.wantStats.Bookmarks || got.Tags != tt.wantStats.Tags {
				t.Fatalf("Stats() = %+v; want %+v", got, tt.wantStats)
			}
		})
	}
}

func TestRepo_StatsFromDB(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("db error")

	tests := []struct {
		name      string
		repoName  string
		dbErr     error
		wantName  string
		wantMarks int
		wantErr   error
	}{
		{
			name:      "success_from_db",
			repoName:  "db-repo",
			dbErr:     nil,
			wantName:  "db-repo",
			wantMarks: 10,
			wantErr:   nil,
		},
		{
			name:      "db_error",
			repoName:  "db-repo",
			dbErr:     dbErr,
			wantName:  "",
			wantMarks: 0,
			wantErr:   dbErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRepo(tt.repoName, t.TempDir())
			db := &mockRepoDB{statsErr: tt.dbErr}

			gotStats, err := r.StatsFromDB(t.Context(), db)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("StatsFromDB() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("StatsFromDB() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("StatsFromDB() unexpected error: %v", err)
			}

			if gotStats.Name != tt.wantName || gotStats.Bookmarks != tt.wantMarks {
				t.Fatalf("StatsFromDB() stats = %+v; want name=%q bookmarks=%d", gotStats, tt.wantName, tt.wantMarks)
			}
		})
	}
}

func TestSummary_Validate(t *testing.T) {
	t.Parallel()

	validStats := &RepoStats{
		Name:      "test-repo",
		Bookmarks: 5,
	}

	tests := []struct {
		name    string
		summary *Summary
		wantErr error
	}{
		{
			name: "valid_summary_with_checksum",
			summary: &Summary{
				GitBranch: "main",
				RepoStats: validStats,
			},
			wantErr: nil,
		},
		{
			name: "empty_checksum_error",
			summary: &Summary{
				GitBranch: "main",
				RepoStats: validStats,
				Checksum:  "",
			},
			wantErr: ErrSummaryChecksumEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantErr == nil && tt.summary.Checksum == "" {
				tt.summary.GenChecksum()
			}

			err := tt.summary.Validate()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Validate() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestSummary_GenChecksum(t *testing.T) {
	t.Parallel()

	s1 := &Summary{
		GitBranch:          "main",
		GitRemote:          "origin",
		ConflictResolution: "merge",
		HashAlgorithm:      "sha256",
		RepoStats: &RepoStats{
			Name:      "repo-name",
			Bookmarks: 10,
			Tags:      2,
		},
		ClientInfo: &ClientInfo{
			Hostname:   "host-1",
			Platform:   "linux",
			Architect:  "amd64",
			AppVersion: "v1.0.0",
		},
	}

	s2 := &Summary{
		GitBranch:          "main",
		GitRemote:          "origin",
		ConflictResolution: "merge",
		HashAlgorithm:      "sha256",
		RepoStats: &RepoStats{
			Name:      "repo-name",
			Bookmarks: 10,
			Tags:      2,
		},
		ClientInfo: &ClientInfo{
			Hostname:   "host-1",
			Platform:   "linux",
			Architect:  "amd64",
			AppVersion: "v1.0.0",
		},
	}

	s1.GenChecksum()
	s2.GenChecksum()

	if s1.Checksum == "" {
		t.Fatalf("GenChecksum() generated an empty checksum")
	}

	if s1.Checksum != s2.Checksum {
		t.Fatalf("GenChecksum() produced inconsistent results for identical summaries: %q vs %q", s1.Checksum, s2.Checksum)
	}
}

func TestRepo_WriteSummary(t *testing.T) {
	t.Parallel()

	validStats := &RepoStats{
		Name:      "test-repo",
		Bookmarks: 5,
	}

	tests := []struct {
		name    string
		summary *Summary
		want    error
		wantErr bool
	}{
		{
			name: "success_write",
			summary: &Summary{
				GitBranch: "main",
				RepoStats: validStats,
			},
		},
		{
			name: "validation_error_nil_stats",
			summary: &Summary{
				GitBranch: "main",
				RepoStats: nil,
			},
			want: ErrSummaryStatsNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			summaryPath := filepath.Join(tempDir, "summary.json")
			r := &Repo{
				name:        tt.name,
				summaryFile: summaryPath,
			}

			err := r.WriteSummary(tt.summary)

			switch {
			case tt.want != nil:
				if !errors.Is(err, tt.want) {
					t.Fatalf("WriteSummary() error = %v, want %v", err, tt.want)
				}
				return
			case tt.wantErr:
				if err == nil {
					t.Fatalf("WriteSummary() expected error, got nil")
				}
				return
			case err != nil:
				t.Fatalf("WriteSummary() unexpected error: %v", err)
			}

			if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
				t.Fatalf("WriteSummary() expected file to be created at %s", summaryPath)
			}
		})
	}
}

func TestSummaryComplete(t *testing.T) {
	t.Parallel()

	errBranch := errors.New("exit status 128: not a git repository")
	hostnameErr := errors.New("hostname err")

	stats := &RepoStats{Name: "test-repo", Bookmarks: 5}

	tests := []struct {
		name        string
		branchOut   string
		branchErr   error
		remoteOut   string
		remoteErr   error
		hostnameErr error
		wantErr     error
		wantBranch  string
		wantRemote  string
	}{
		{
			name:       "success_with_remote",
			branchOut:  "main\n",
			remoteOut:  "git@github.com:mateconpizza/gm.git\n",
			wantBranch: "main",
			wantRemote: "git@github.com:mateconpizza/gm.git",
		},
		{
			name:       "remote_error_falls_back_to_empty",
			branchOut:  "main\n",
			remoteErr:  errors.New("exit status 1: no such remote"),
			wantBranch: "main",
			wantRemote: "",
		},
		{
			name:       "no_remote_configured_empty_output",
			branchOut:  "main\n",
			remoteOut:  "",
			wantBranch: "main",
			wantRemote: "",
		},
		{
			name:      "branch_error_propagates",
			branchErr: errBranch,
			wantErr:   errBranch,
		},
		{
			name:        "hostname_err",
			hostnameErr: hostnameErr,
			wantErr:     hostnameErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{}).
				onContains("HEAD", tt.branchOut, tt.branchErr).
				on("config", tt.remoteOut, tt.remoteErr)

			g, err := New(t.TempDir(), WithExecuter(fake.run))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			hostnameFunc := func() (string, error) {
				return "hostname", tt.hostnameErr
			}

			before := time.Now()
			sum, err := summaryComplete(t.Context(), g, stats, hostnameFunc, "1.2.3")
			after := time.Now()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("summaryComplete() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("summaryComplete() unexpected error: %v", err)
			}

			if sum.GitBranch != tt.wantBranch {
				t.Errorf("GitBranch = %q, want %q", sum.GitBranch, tt.wantBranch)
			}
			if sum.GitRemote != tt.wantRemote {
				t.Errorf("GitRemote = %q, want %q", sum.GitRemote, tt.wantRemote)
			}
			if sum.ConflictResolution != "timestamp" {
				t.Errorf("ConflictResolution = %q, want %q", sum.ConflictResolution, "timestamp")
			}
			if sum.HashAlgorithm != "SHA-256" {
				t.Errorf("HashAlgorithm = %q, want %q", sum.HashAlgorithm, "SHA-256")
			}
			if sum.RepoStats != stats {
				t.Errorf("RepoStats = %v, want %v (same pointer)", sum.RepoStats, stats)
			}
			if sum.Checksum == "" {
				t.Error("Checksum is empty, want GenChecksum() to have populated it")
			}

			if sum.ClientInfo == nil {
				t.Fatal("ClientInfo is nil")
			}

			wantHostname, _ := hostnameFunc()
			if sum.ClientInfo.Hostname != wantHostname {
				t.Errorf("ClientInfo.Hostname = %q, want %q", sum.ClientInfo.Hostname, wantHostname)
			}
			if sum.ClientInfo.Platform != runtime.GOOS {
				t.Errorf("ClientInfo.Platform = %q, want %q", sum.ClientInfo.Platform, runtime.GOOS)
			}
			if sum.ClientInfo.Architect != runtime.GOARCH {
				t.Errorf("ClientInfo.Architect = %q, want %q", sum.ClientInfo.Architect, runtime.GOARCH)
			}
			if sum.ClientInfo.AppVersion != "1.2.3" {
				t.Errorf("ClientInfo.AppVersion = %q, want %q", sum.ClientInfo.AppVersion, "1.2.3")
			}

			gotSync, parseErr := time.Parse(time.RFC3339, sum.LastSync)
			if parseErr != nil {
				t.Fatalf("LastSync = %q, not valid RFC3339: %v", sum.LastSync, parseErr)
			}
			if gotSync.Before(before.Truncate(time.Second)) || gotSync.After(after.Add(time.Second)) {
				t.Errorf("LastSync = %v, want between %v and %v", gotSync, before, after)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    int
		wantErr bool
	}{
		{
			name: "valid",
			data: `{"bookmarks":42}`,
			want: 42,
		},
		{
			name:    "invalid",
			data:    `[invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Bookmarks int `json:"bookmarks"`
			}

			err := decodeJSON([]byte(tt.data), &got)

			if tt.wantErr {
				var syntaxErr *json.SyntaxError
				if !errors.As(err, &syntaxErr) {
					t.Fatalf("decodeJSON() error = %v, want *json.SyntaxError", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("decodeJSON() unexpected error: %v", err)
			}

			if got.Bookmarks != tt.want {
				t.Errorf("Bookmarks = %d, want %d", got.Bookmarks, tt.want)
			}
		})
	}
}
