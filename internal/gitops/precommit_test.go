package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

func TestHookSummaryWrite(t *testing.T) {
	t.Parallel()

	errStatus := errors.New("status failed")
	errLoad := errors.New("stats unavailable")

	// summaryJSON builds a stored summary with the given stats.
	summaryJSON := func(name string, bookmarks, tags, favorites, archived, dead, visits int) string {
		return fmt.Sprintf(
			`{"stats":{"name":%q,"bookmarks":%d,"tags":%d,"favorites":%d,"archived":%d,"dead_links":%d,"total_visits":%d}}`,
			name, bookmarks, tags, favorites, archived, dead, visits,
		)
	}

	fresh := git.RepoStats{
		Name: "repo", Bookmarks: 10, Tags: 26, Favorites: 2, Archived: 2, DeadLinks: 0, TotalVisits: 32,
	}
	sameSummary := summaryJSON("repo", 10, 26, 2, 2, 0, 32)
	oldSummary := summaryJSON("repo", 9, 26, 2, 2, 0, 32)

	tests := []struct {
		name        string
		summary     string
		status      string
		statusErr   error
		loadErr     error
		wantErr     error
		wantErrMsg  string
		wantWritten bool
	}{
		{
			name:        "dirty_tree_no_summary_writes",
			status:      " M main.json\n",
			wantWritten: true,
		},
		{
			name:        "untracked_files_write",
			status:      "?? new.json\n",
			wantWritten: true,
		},
		{
			name:        "dirty_tree_replaces_existing_summary",
			summary:     oldSummary,
			status:      " M main.json\n",
			wantWritten: true,
		},
		{
			name:        "dirty_tree_same_stats_still_writes",
			summary:     sameSummary,
			status:      " M main.json\n",
			wantWritten: true,
		},
		{
			name:        "clean_tree_no_summary_writes",
			wantWritten: true,
		},
		{
			name:        "clean_tree_same_stats_is_up_to_date",
			summary:     sameSummary,
			wantErr:     git.ErrGitUpToDate,
			wantWritten: false,
		},
		{
			name:        "clean_tree_stats_differ_writes",
			summary:     oldSummary,
			wantWritten: true,
		},
		{
			name:        "whitespace_only_status_is_clean",
			summary:     sameSummary,
			status:      " \n",
			wantErr:     git.ErrGitUpToDate,
			wantWritten: false,
		},
		{
			name:      "git_status_fails",
			summary:   sameSummary,
			statusErr: errStatus,
			wantErr:   errStatus,
		},
		{
			name:       "stats_loader_fails",
			summary:    sameSummary,
			status:     " M main.json\n",
			loadErr:    errLoad,
			wantErr:    errLoad,
			wantErrMsg: "fresh stats",
		},
		{
			name:    "context_canceled",
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			summaryPath := filepath.Join(dir, git.SummaryFileName)
			if tt.summary != "" {
				testutil.NewFile(t, dir, git.SummaryFileName, []byte(tt.summary))
			}

			gitRunner := testutil.NewFakeGitExecuter().
				On("status", tt.status, tt.statusErr).
				Run

			g := testutil.NewGit(t, t.TempDir(), git.WithExecuter(gitRunner))
			gr := git.NewRepo("repo", dir)

			stats := fresh
			hook := HookSummaryWrite(gr, "test", func(ctx context.Context) (*git.RepoStats, error) {
				if tt.loadErr != nil {
					return nil, tt.loadErr
				}
				return &stats, nil
			})

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := hook.Run(ctx, g)

			if tt.wantErr == nil && tt.wantErrMsg == "" {
				if err != nil {
					t.Fatalf("PreCommitSummaryWrite() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("PreCommitSummaryWrite() expected error %v %q, got nil", tt.wantErr, tt.wantErrMsg)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("PreCommitSummaryWrite() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("PreCommitSummaryWrite() error = %v; want it to contain %q", err, tt.wantErrMsg)
				}
			}

			if tt.wantWritten {
				got, err := gr.Summary()
				if err != nil {
					t.Fatalf("reading written summary: %v", err)
				}
				if got.RepoStats == nil || *got.RepoStats != fresh {
					t.Fatalf("written stats = %+v; want %+v", got.RepoStats, fresh)
				}
				return
			}

			after, readErr := os.ReadFile(summaryPath)
			switch {
			case tt.summary == "":
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("summary file should not exist, read error = %v", readErr)
				}
			case readErr != nil || string(after) != tt.summary:
				t.Fatalf("summary file changed: err = %v, got %q; want %q", readErr, after, tt.summary)
			}
		})
	}
}

func TestHookReadme(t *testing.T) {
	t.Parallel()

	var (
		errBookmarks = errors.New("bookmarks failed")
		errStats     = errors.New("stats failed")
		errCipher    = errors.New("cipher failed")
	)

	bms := []*bookmark.Bookmark{{}}
	stats := &git.RepoStats{}

	tests := []struct {
		name         string
		bms          []*bookmark.Bookmark
		cancelCtx    bool
		bookmarksErr error
		statsErr     error
		cipherErr    error
		readErr      error
		existing     string // "", "same" or "different"
		wantWrites   int
		wantErr      error
	}{
		{name: "missing_file_writes", bms: bms, wantWrites: 1},
		{name: "empty_bookmarks_writes", bms: nil, wantWrites: 1},
		{name: "same_content_skips_write", bms: bms, existing: "same", wantWrites: 0},
		{name: "different_content_overwrites", bms: bms, existing: "different", wantWrites: 1},
		{name: "ctx_cancelled", bms: bms, cancelCtx: true, wantErr: context.Canceled},
		{name: "bookmarks_error", bms: bms, bookmarksErr: errBookmarks, wantErr: errBookmarks},
		{name: "stats_error", bms: bms, statsErr: errStats, wantErr: errStats},
		{name: "cipher_error", bms: bms, cipherErr: errCipher, wantErr: errCipher},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &ReadmeCfg{
				Name: "test",
				Path: filepath.Join(t.TempDir(), "sub", "README.md"),
			}

			fc := testutil.NewFakeCipher(".fake")
			fullpath := cfg.Path + fc.Suffix()

			var want bytes.Buffer
			if err := NewReadmeFile(&want, cfg, stats, tt.bms); err != nil {
				t.Fatalf("NewReadmeFile: %v", err)
			}

			if tt.existing != "" {
				if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullpath, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				content := []byte("stale")
				if tt.existing == "same" {
					content = want.Bytes()
				}
				fc.Put(fullpath, content)
			}

			bookmarks := func(context.Context) ([]*bookmark.Bookmark, error) {
				return tt.bms, tt.bookmarksErr
			}
			statsLoader := func(context.Context) (*git.RepoStats, error) {
				return stats, tt.statsErr
			}
			cipher := func(context.Context) (enc.Cipher, error) {
				if tt.cipherErr != nil {
					return nil, tt.cipherErr
				}
				return fc, nil
			}

			h := HookReadme(cfg, bookmarks, statsLoader, cipher)
			if h.Name != "readme" {
				t.Fatalf("Hook.Name = %q; want %q", h.Name, "readme")
			}

			g := testutil.NewGit(
				t,
				t.TempDir(),
				git.WithExecuter(testutil.NewFakeGitExecuter().Run),
			)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancelCtx {
				cancel()
			}

			err := h.Run(ctx, g)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Run() error = %v; want %v", err, tt.wantErr)
				}
				if fc.Writes() != 0 {
					t.Fatalf("Run() wrote %d times on error; want 0", fc.Writes())
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}

			if fc.Writes() != tt.wantWrites {
				t.Fatalf("writes = %d; want %d", fc.Writes(), tt.wantWrites)
			}
			if got, _ := fc.Get(fullpath); !bytes.Equal(got, want.Bytes()) {
				t.Fatalf("stored README = %q; want %q", got, want.Bytes())
			}
		})
	}
}

func TestHookHasChanges(t *testing.T) {
	t.Parallel()

	errStatus := errors.New("status failed")

	tests := []struct {
		name      string
		status    string // output of `git status --porcelain`
		statusErr error
		wantErr   error
	}{
		{"modified_file", " M main.json\n", nil, nil},
		{"staged_file", "A  new.json\n", nil, nil},
		{"untracked_file", "?? new.json\n", nil, nil},
		{"multiple_changes", " M a.json\n?? b.json\nD  c.json\n", nil, nil},
		{"single_line_without_newline", " M main.json", nil, nil},
		{"clean_tree", "", nil, git.ErrGitUpToDate},
		{"only_newline", "\n", nil, git.ErrGitUpToDate},
		{"only_whitespace", " \t\n", nil, git.ErrGitUpToDate},
		{"git_status_fails", "", errStatus, errStatus},
		// output is ignored when git fails, so it can't turn the error into success
		{"git_status_fails_with_output", " M main.json\n", errStatus, errStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := testutil.NewFakeGitExecuter().
				On("status", tt.status, tt.statusErr).
				Run
			g := testutil.NewGit(t, t.TempDir(), git.WithExecuter(runner))

			hook := HookHasChanges()
			if hook.Name != "changes" {
				t.Fatalf("hook.Name = %q; want %q", hook.Name, "changes")
			}

			err := hook.Run(t.Context(), g)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("PreCommitHasChanges() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PreCommitHasChanges() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				// a git failure must not be reported as "up to date"
				if tt.statusErr != nil && errors.Is(err, git.ErrGitUpToDate) {
					t.Fatalf("PreCommitHasChanges() error = %v; must not match ErrGitUpToDate", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("PreCommitHasChanges() unexpected error: %v", err)
			}
		})
	}
}
