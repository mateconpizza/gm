package git

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type fakeRepoDB struct {
	bookmarks int
	err       error
}

func (f *fakeRepoDB) Stats(ctx context.Context, dest any) error {
	if f.err != nil {
		return f.err
	}
	if rs, ok := dest.(*RepoStats); ok {
		rs.Bookmarks = f.bookmarks
	}
	return nil
}

func newTestMgrRepo(t *testing.T, fake *fakeGitExecuter, db *fakeRepoDB, version string) (*Mgr, *Repo) {
	t.Helper()

	g, err := New(t.TempDir(), WithExecuter(fake.run))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	m, err := NewManager(t.TempDir(), WithGit(g), WithVersion(version))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	var opts []RepoOptFunc
	if db != nil {
		opts = append(opts, WithRepoStore(db))
	}
	gr := m.NewRepo("myrepo", opts...)

	return m, gr
}

func TestMgr_SaveChanges(t *testing.T) {
	t.Parallel()

	errDB := errors.New("db unavailable")
	errStatus := errors.New("exit status 128")

	tests := []struct {
		name              string
		version           string
		noDB              bool
		dbBookmarks       int
		dbErr             error
		statusOut         string
		statusErr         error
		branchErr         error
		commitOut         string
		commitErr         error
		seedMatchingStats bool // pre-write a summary file so oldStats == freshStats
		want              error
		wantErrMsg        string
	}{
		{
			name:    "missing_version",
			version: "",
			want:    ErrNoVersionFound,
		},
		{
			name:    "no_db_configured",
			version: "1.0.0",
			noDB:    true,
			want:    ErrNoFunctionFound,
		},
		{
			name:        "stats_from_db_fails",
			version:     "1.0.0",
			dbBookmarks: 3,
			dbErr:       errDB,
			want:        errDB,
		},
		{
			name:        "has_changes_check_fails",
			version:     "1.0.0",
			dbBookmarks: 3,
			statusErr:   errStatus,
			wantErrMsg:  "git status failed",
		},
		{
			name:              "up_to_date_no_changes_equal_stats",
			version:           "1.0.0",
			dbBookmarks:       5,
			statusOut:         "",
			seedMatchingStats: true,
			want:              ErrGitUpToDate,
		},
		{
			name:        "branch_lookup_fails_during_summary",
			version:     "1.0.0",
			dbBookmarks: 5,
			statusOut:   " M file.txt\n",
			branchErr:   errors.New("exit status 128"),
			wantErrMsg:  "getting branch",
		},
		{
			name:        "commit_fails",
			version:     "1.0.0",
			dbBookmarks: 5,
			statusOut:   " M file.txt\n",
			commitOut:   "fatal: unable to write commit object",
			commitErr:   errors.New("exit status 1"),
			wantErrMsg:  "commit: exit status 1",
		},
		{
			name:        "success_full_flow",
			version:     "1.0.0",
			dbBookmarks: 5,
			statusOut:   " M file.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{}).
				on("status", tt.statusOut, tt.statusErr).
				onContains("HEAD", "main\n", tt.branchErr).
				on("config", "", nil).
				on("commit", tt.commitOut, tt.commitErr)

			var db *fakeRepoDB
			if !tt.noDB {
				db = &fakeRepoDB{bookmarks: tt.dbBookmarks, err: tt.dbErr}
			}

			m, gr := newTestMgrRepo(t, fake, db, tt.version)

			if tt.seedMatchingStats {
				seedStats := &RepoStats{Name: gr.Name(), Bookmarks: tt.dbBookmarks}
				if err := gr.WriteSummary(&Summary{RepoStats: seedStats}); err != nil {
					t.Fatalf("setup: seeding summary: %v", err)
				}
			}

			err := m.SaveChanges(t.Context(), gr, "commit message")

			switch {
			case tt.want != nil:
				if !errors.Is(err, tt.want) {
					t.Fatalf("SaveChanges() error = %v, want %v", err, tt.want)
				}
			case tt.wantErrMsg != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("SaveChanges() error = %v, want containing %q", err, tt.wantErrMsg)
				}
			case err != nil:
				t.Fatalf("SaveChanges() unexpected error: %v", err)
			}
		})
	}
}

func TestMgr_shouldSave(t *testing.T) {
	t.Parallel()

	statsA := &RepoStats{Name: "repo", Bookmarks: 5}
	statsB := &RepoStats{Name: "repo", Bookmarks: 5} // deep-equal to statsA
	statsC := &RepoStats{Name: "repo", Bookmarks: 9} // differs

	errStatus := errors.New("exit status 128")

	tests := []struct {
		name       string
		statusOut  string
		statusErr  error
		old, fresh *RepoStats
		want       bool
		wantErr    error
	}{
		{
			name:      "changed_true_overrides_equal_stats",
			statusOut: " M file.txt\n",
			old:       statsA,
			fresh:     statsB,
			want:      true,
		},
		{
			name:      "unchanged_and_equal_stats_no_save_needed",
			statusOut: "",
			old:       statsA,
			fresh:     statsB,
			want:      false,
		},
		{
			name:      "unchanged_but_differing_stats_needs_save",
			statusOut: "",
			old:       statsA,
			fresh:     statsC,
			want:      true,
		},
		{
			name:      "changed_and_differing_stats_needs_save",
			statusOut: " M file.txt\n",
			old:       statsA,
			fresh:     statsC,
			want:      true,
		},
		{
			name:      "has_changes_error_propagates",
			statusErr: errStatus,
			old:       statsA,
			fresh:     statsA,
			wantErr:   errStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{}).on("status", tt.statusOut, tt.statusErr)

			g, err := New(t.TempDir(), WithExecuter(fake.run))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			m, err := NewManager(t.TempDir(), WithGit(g), WithVersion("1.0.0"))
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			got, err := m.shouldSave(t.Context(), tt.old, tt.fresh)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("needsSave() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("needsSave() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("needsSave() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMgr_Untrack(t *testing.T) {
	t.Parallel()

	errCommit := errors.New("exit status 1")

	tests := []struct {
		name       string
		repoName   string
		skipTrack  bool // if true, never call m.Track before Untrack
		commitOut  string
		commitErr  error
		want       error
		wantErrMsg string
	}{
		{
			name:      "not_tracked_returns_error",
			repoName:  "myrepo",
			skipTrack: true,
			want:      ErrGitNotTracked,
		},
		{
			name:       "commit_fails_after_untrack",
			repoName:   "myrepo",
			commitOut:  "fatal: unable to write commit object",
			commitErr:  errCommit,
			wantErrMsg: "commit: exit status 1",
		},
		{
			name:     "success_untracks_and_removes_dir",
			repoName: "myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{}).
				on("status", " M file.txt\n", nil).
				on("commit", tt.commitOut, tt.commitErr)

			m, gr := newTestMgrRepo(t, fake, nil, "1.0.0")

			if !tt.skipTrack {
				if err := m.Track(tt.repoName); err != nil {
					t.Fatalf("setup: Track() error = %v", err)
				}
			}

			err := m.Untrack(t.Context(), gr, "untrack message")

			switch {
			case tt.want != nil:
				if !errors.Is(err, tt.want) {
					t.Fatalf("Untrack() error = %v, want %v", err, tt.want)
				}
				return
			case tt.wantErrMsg != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Untrack() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			case err != nil:
				t.Fatalf("Untrack() unexpected error: %v", err)
			}

			if m.IsTracked(tt.repoName) {
				t.Error("repo still tracked after successful Untrack()")
			}
			if _, statErr := os.Stat(gr.Fullpath()); !os.IsNotExist(statErr) {
				t.Errorf("repo dir at %s still exists after Untrack()", gr.Fullpath())
			}
		})
	}
}

func TestMgr_Update(t *testing.T) {
	t.Parallel()

	errRemove := errors.New("disk error removing file")
	errWrite := errors.New("disk error writing file")
	errPostRm := errors.New("cleanup failed")

	old := &bookmark.Bookmark{ID: 1, URL: "https://old.com"}
	fresh := &bookmark.Bookmark{ID: 2, URL: "https://fresh.com"}

	tests := []struct {
		name        string
		version     string
		noDB        bool
		removerErr  error // nil will succeeds
		postRmErr   error
		writerErr   error
		want        error
		wantErrMsg  string
		wantFresh   bool // fresh should end up in gr.Bookmarks()
		wantOldGone bool // old should be gone from gr.Bookmarks()
	}{
		{
			name:    "missing_version",
			version: "",
			want:    ErrNoVersionFound,
		},
		{
			name:    "no_store_configured",
			version: "1.0.0",
			noDB:    true,
			want:    ErrNoStoreFound,
		},
		{
			name:       "remove_fails_non_not_exist",
			version:    "1.0.0",
			removerErr: errRemove,
			want:       errRemove,
		},
		{
			name:      "post_removal_fails_non_not_exist",
			version:   "1.0.0",
			postRmErr: errPostRm,
			want:      errPostRm,
		},
		{
			name:        "remove_not_exist_is_ignored_then_adds",
			version:     "1.0.0",
			removerErr:  os.ErrNotExist,
			wantFresh:   true,
			wantOldGone: false,
		},
		{
			name:      "add_fails_after_successful_remove",
			version:   "1.0.0",
			writerErr: errWrite,
			want:      errWrite,
		},
		{
			name:        "success_removes_old_adds_fresh",
			version:     "1.0.0",
			wantFresh:   true,
			wantOldGone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{})
			g, err := New(t.TempDir(), WithExecuter(fake.run))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			m, err := NewManager(t.TempDir(), WithGit(g), WithVersion(tt.version))
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			opts := []RepoOptFunc{
				WithRepoRemover(func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
					return tt.removerErr
				}),
				WithRepoWriter(func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
					return tt.writerErr
				}),
			}
			if !tt.noDB {
				opts = append(opts, WithRepoStore(&fakeRepoDB{}))
			}

			gr := m.NewRepo("myrepo", opts...)
			gr.bookmarks = []*bookmark.Bookmark{old}

			postRm := func(path string) error { return tt.postRmErr }

			err = m.Update(t.Context(), gr, old, fresh, postRm)

			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("Update() error = %v, want %v", err, tt.want)
				}
				return
			}
			if tt.wantErrMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Update() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() unexpected error: %v", err)
			}

			hasID := func(id int) bool {
				return slices.ContainsFunc(gr.Bookmarks(), func(b *bookmark.Bookmark) bool {
					return b.ID == id
				})
			}

			if got := hasID(fresh.ID); got != tt.wantFresh {
				t.Errorf("fresh present in bookmarks = %v, want %v", got, tt.wantFresh)
			}
			if got := !hasID(old.ID); got != tt.wantOldGone {
				t.Errorf("old absent from bookmarks = %v, want %v", got, tt.wantOldGone)
			}
		})
	}
}
