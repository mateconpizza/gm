package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestMgrRepo(t *testing.T, fake *fakeGitExecuter) (*Mgr, *Repo) {
	t.Helper()

	g, err := New(t.TempDir(), WithExecuter(fake.run))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	m, err := NewManager(t.TempDir(), WithGit(g))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	var opts []RepoOptFunc
	gr := m.NewRepo("myrepo", opts...)

	return m, gr
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
			wantErrMsg: "exit status 1",
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

			m, gr := newTestMgrRepo(t, fake)

			if !tt.skipTrack {
				if err := m.Track(tt.repoName); err != nil {
					t.Fatalf("setup: Track() error = %v", err)
				}
			}

			err := m.Untrack(t.Context(), gr)

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

func TestMgr_Push(t *testing.T) {
	t.Parallel()

	errTestBoom := errors.New("test execution error")

	tests := []struct {
		name              string
		configOut         string
		configErr         error
		setUpstreamOut    string
		setUpstreamErr    error
		hasUnpushedComOut string
		hasUnpushedComErr error
		pushErr           error
		wantErr           error
	}{
		{
			name:              "happy_path_successful_push",
			configOut:         "git@github.com:user/repo.git",
			hasUnpushedComOut: "3",
			wantErr:           nil,
		},
		{
			name:      "no_upstream_empty_remote",
			configOut: "",
			wantErr:   ErrGitNoUpstream,
		},
		{
			name:      "no_upstream_remote_error",
			configErr: errTestBoom,
			wantErr:   ErrGitNoUpstream,
		},
		{
			name:              "set_upstream_exists_continues",
			configOut:         "origin",
			setUpstreamErr:    ErrGitUpstreamExists,
			hasUnpushedComOut: "1",
			wantErr:           nil,
		},
		{
			name:              "has_unpushed_commits_error",
			configOut:         "origin",
			hasUnpushedComErr: errTestBoom,
			wantErr:           errTestBoom,
		},
		{
			name:              "up_to_date_no_commits",
			configOut:         "origin",
			hasUnpushedComOut: "0",
			wantErr:           ErrGitUpToDate,
		},
		{
			name:              "final_push_fails",
			configOut:         "origin",
			hasUnpushedComOut: "2",
			pushErr:           errTestBoom,
			wantErr:           errTestBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := (&fakeGitExecuter{}).
				onContains("remote.origin.url", tt.configOut, tt.configErr).
				on("remote", tt.configOut, tt.configErr).
				onContains("set-upstream", tt.setUpstreamOut, tt.setUpstreamErr).
				on("rev-list", tt.hasUnpushedComOut, tt.hasUnpushedComErr).
				on("push", "", tt.pushErr)

			m, _ := newTestMgrRepo(t, fake)

			err := m.Push(t.Context())

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Push() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Push() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Push() unexpected error: %v", err)
			}
		})
	}
}

func TestMgr_Rename(t *testing.T) {
	t.Parallel()

	errRename := errors.New("rename failed")

	tests := []struct {
		name    string
		oldName string
		newName string
		tracked bool
		rename  func(src, dest string) error
		wantErr error
	}{
		{
			name:    "success",
			oldName: "main.db",
			newName: "archive.db",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: nil,
		},
		{
			name:    "untracked_repo",
			oldName: "main.db",
			newName: "archive.db",
			tracked: false,
			rename:  func(src, dest string) error { return nil },
			wantErr: nil,
		},
		{
			name:    "same_name",
			oldName: "main.db",
			newName: "main.db",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: nil,
		},
		{
			name:    "same_base_name_different_extension",
			oldName: "main.db",
			newName: "main.sqlite",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: nil,
		},
		{
			name:    "empty_old_name",
			oldName: "",
			newName: "archive.db",
			tracked: false,
			rename:  func(src, dest string) error { return nil },
			wantErr: os.ErrInvalid,
		},
		{
			name:    "empty_new_name",
			oldName: "main.db",
			newName: "",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: os.ErrInvalid,
		},
		{
			name:    "rename_error",
			oldName: "main.db",
			newName: "archive.db",
			tracked: true,
			rename:  func(src, dest string) error { return errRename },
			wantErr: errRename,
		},
		{
			name:    "extensionless_names",
			oldName: "main",
			newName: "archive",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: nil,
		},
		{
			name:    "context_canceled",
			oldName: "palan",
			newName: "palandri",
			tracked: true,
			rename:  func(src, dest string) error { return nil },
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()

			gitFake := &fakeGitExecuter{}
			g, err := New(
				root,
				WithExecuter(gitFake.run),
			)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}

			mgr, err := NewManager(
				root,
				WithGit(g),
			)
			if err != nil {
				t.Fatalf("NewManager() unexpected error: %v", err)
			}

			if tt.tracked {
				if err := mgr.Track(strings.TrimSuffix(tt.oldName, filepath.Ext(tt.oldName))); err != nil {
					t.Fatalf("Track() unexpected error: %v", err)
				}
			}

			var gotSrc, gotDest string

			mgr.fileMgr.rename = func(src, dest string) error {
				gotSrc = src
				gotDest = dest
				return tt.rename(src, dest)
			}

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err = mgr.Rename(ctx, tt.oldName, tt.newName)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Rename() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Rename() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Rename() unexpected error: %v", err)
			}

			if !tt.tracked || strings.TrimSuffix(tt.oldName, filepath.Ext(tt.oldName)) ==
				strings.TrimSuffix(tt.newName, filepath.Ext(tt.newName)) {
				if gotSrc != "" || gotDest != "" {
					t.Fatalf("Rename() called renameFn unexpectedly: %q -> %q", gotSrc, gotDest)
				}
				return
			}

			oldBase := strings.TrimSuffix(tt.oldName, filepath.Ext(tt.oldName))
			newBase := strings.TrimSuffix(tt.newName, filepath.Ext(tt.newName))

			wantSrc := filepath.Join(root, oldBase)
			wantDest := filepath.Join(root, newBase)

			if gotSrc != wantSrc {
				t.Fatalf("Rename() source = %q; want %q", gotSrc, wantSrc)
			}
			if gotDest != wantDest {
				t.Fatalf("Rename() destination = %q; want %q", gotDest, wantDest)
			}
		})
	}
}
