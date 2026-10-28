package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	errBookmarks := errors.New("bookmarks unavailable")
	errStats := errors.New("stats unavailable")

	bs := func(n int) []*bookmark.Bookmark {
		out := make([]*bookmark.Bookmark, 0, n)
		for i := range n {
			out = append(out, &bookmark.Bookmark{
				ID:    i + 1,
				URL:   fmt.Sprintf("https://example%d.com", i),
				Title: fmt.Sprintf("Title %d", i),
			})
		}
		return out
	}

	tests := []struct {
		name         string
		enabled      bool
		cancelCtx    bool
		bookmarks    int
		bookmarksErr error
		statsErr     error

		wantErr          error
		wantErrMsg       string
		wantBookmarkCall int // times the bookmarks loader ran
		wantStatsCall    int // times the stats loader ran
		wantFile         bool
		wantContent      bool // the README must not be empty
	}{
		{
			name: "typical_writes_readme", enabled: true, bookmarks: 3,
			wantBookmarkCall: 1, wantStatsCall: 1, wantFile: true, wantContent: true,
		},
		{
			name: "single_bookmark", enabled: true, bookmarks: 1,
			wantBookmarkCall: 1, wantStatsCall: 1, wantFile: true, wantContent: true,
		},
		{
			name: "empty_bookmarks_still_writes", enabled: true, bookmarks: 0,
			wantBookmarkCall: 1, wantStatsCall: 1, wantFile: true, wantContent: true,
		},
		{
			name: "disabled_does_nothing", enabled: false, bookmarks: 3,
		},
		{
			name: "canceled_context", enabled: true, cancelCtx: true, bookmarks: 3,
			wantErr: context.Canceled,
		},
		// NOTE: the README file is opened (and truncated) before the loaders
		// run, so a failing loader leaves an empty file behind. This
		// documents the current behaviour.
		{
			name: "bookmarks_loader_fails", enabled: true, bookmarks: 3, bookmarksErr: errBookmarks,
			wantErr: errBookmarks, wantErrMsg: "load bookmarks",
			wantBookmarkCall: 1, wantFile: true,
		},
		{
			name: "stats_loader_fails", enabled: true, bookmarks: 3, statsErr: errStats,
			wantErr:          errStats,
			wantBookmarkCall: 1, wantStatsCall: 1, wantFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testutil.NewApp(t)
			app.Git.Readme.Enabled = tt.enabled

			path := app.Path.GitReadme(app.DBBaseName())
			testutil.NewDir(t, filepath.Dir(path))

			ctx := t.Context()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			var bookmarkCalls, statsCalls int

			loadBookmarks := func(ctx context.Context) ([]*bookmark.Bookmark, error) {
				bookmarkCalls++
				if tt.bookmarksErr != nil {
					return nil, tt.bookmarksErr
				}
				return bs(tt.bookmarks), nil
			}
			loadStats := func(ctx context.Context) (*git.RepoStats, error) {
				statsCalls++
				if tt.statsErr != nil {
					return nil, tt.statsErr
				}
				return &git.RepoStats{
					Name: app.DBBaseName(), Bookmarks: tt.bookmarks, Tags: 26, Favorites: 2,
					Archived: 2, DeadLinks: 0, TotalVisits: 32,
				}, nil
			}

			g := testutil.NewGit(t, t.TempDir())

			hook := HookReadme(app, loadBookmarks, loadStats)
			if hook.Name != "readme" {
				t.Fatalf("hook.Name = %q; want %q", hook.Name, "readme")
			}

			err := hook.Run(ctx, g)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("PreCommitReadme() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PreCommitReadme() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("PreCommitReadme() error = %v; want it to contain %q", err, tt.wantErrMsg)
				}
			} else if err != nil {
				t.Fatalf("PreCommitReadme() unexpected error: %v", err)
			}

			if bookmarkCalls != tt.wantBookmarkCall {
				t.Fatalf("bookmarks loader calls = %d; want %d", bookmarkCalls, tt.wantBookmarkCall)
			}
			if statsCalls != tt.wantStatsCall {
				t.Fatalf("stats loader calls = %d; want %d", statsCalls, tt.wantStatsCall)
			}

			data, readErr := os.ReadFile(path)
			if !tt.wantFile {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("README should not exist, read error = %v", readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatalf("reading README: %v", readErr)
			}
			if tt.wantContent && len(data) == 0 {
				t.Fatal("README is empty; want rendered content")
			}
			if !tt.wantContent && len(data) != 0 {
				t.Fatalf("README = %q; want an empty file after a failed load", data)
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
