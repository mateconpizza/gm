package gitops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type fakeRepoDB struct {
	bookmarks []*bookmark.Bookmark
	total     int
	err       error
}

func (f *fakeRepoDB) All(ctx context.Context) ([]*bookmark.Bookmark, error) { return f.bookmarks, nil }

func (f *fakeRepoDB) Stats(ctx context.Context, dest any) error {
	if f.err != nil {
		return f.err
	}
	if rs, ok := dest.(*git.RepoStats); ok {
		rs.Bookmarks = f.total
	}
	return nil
}

func newTestMgrRepo(t *testing.T, fake *testutil.FakeGitExecuter, db *fakeRepoDB, version string, opts ...git.RepoOptFunc) (*git.Mgr, *git.Repo) {
	t.Helper()

	tempDir := t.TempDir()
	g, err := git.New(tempDir, git.WithExecuter(fake.Run))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	gm, err := git.NewManager(tempDir, git.WithGit(g), git.WithVersion(version))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if db != nil {
		opts = append(opts, git.WithRepoStore(db))
	}
	gr := gm.NewRepo("myrepo", opts...)

	return gm, gr
}

func TestTrack(t *testing.T) {
	t.Parallel()

	errGitAdd := errors.New("git add error")
	errGitCommit := errors.New("git commit error")
	errGitStats := errors.New("stats error")
	errRepoWriter := errors.New("writing repository")

	tests := []struct {
		name       string
		db         *fakeRepoDB
		setupFake  func(*testutil.FakeGitExecuter)
		preTrack   bool
		opts       []git.RepoOptFunc
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "normal_success",
			db:   &fakeRepoDB{total: 5},
			opts: []git.RepoOptFunc{RepoFileWriter(false)},
		},
		{
			name: "empty_bookmarks",
			db:   &fakeRepoDB{total: 0, bookmarks: nil},
			opts: []git.RepoOptFunc{RepoFileWriter(false)},
		},
		{
			name:     "already_tracked",
			db:       &fakeRepoDB{total: 1},
			preTrack: true,
			opts:     []git.RepoOptFunc{RepoFileWriter(false)},
			wantErr:  git.ErrGitTracked,
		},
		{
			name:    "stats_error",
			db:      &fakeRepoDB{err: errGitStats},
			opts:    []git.RepoOptFunc{RepoFileWriter(false)},
			wantErr: errGitStats,
		},
		{
			name: "git_add_failure",
			db:   &fakeRepoDB{total: 2},
			setupFake: func(f *testutil.FakeGitExecuter) {
				f.On("status", "changed", nil)
				f.On("add", "", errGitAdd)
			},
			opts:    []git.RepoOptFunc{RepoFileWriter(false)},
			wantErr: errGitAdd,
		},
		{
			name: "git_commit_failure",
			db:   &fakeRepoDB{total: 2},
			setupFake: func(f *testutil.FakeGitExecuter) {
				f.On("status", "changed", nil)
				f.On("commit", "", errGitCommit)
			},
			opts:    []git.RepoOptFunc{RepoFileWriter(false)},
			wantErr: errGitCommit,
		},
		{
			name: "repo_add_failure",
			db:   &fakeRepoDB{},
			opts: []git.RepoOptFunc{git.WithRepoWriter(func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return errRepoWriter
			})},
			wantErr: errRepoWriter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := &testutil.FakeGitExecuter{}
			if tt.setupFake != nil {
				tt.setupFake(fake)
			}

			gm, gr := newTestMgrRepo(t, fake, tt.db, "1.0.0", tt.opts...)

			if tt.preTrack {
				if err := gm.Track(gr.Name()); err != nil {
					t.Fatalf("setup failed: could not pre-track repo: %v", err)
				}
			}

			err := Track(t.Context(), tt.db, gm, gr)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Track() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("Track() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Track() error = %v, wantErrMsg %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Track() unexpected error: %v", err)
			}

			if !gm.IsTracked(gr.Name()) {
				t.Errorf("Track() expected repo %q to be tracked, but it was not", gr.Name())
			}
		})
	}
}
