package gitops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type mockManager struct {
	isEnabled bool
	isTracked bool

	saveChangesErr error
	savedMsg       string

	dropErr error

	trackedName string
	untrackErr  error
	untrackMsg  string

	updateAndSaveErr    error
	updateAndSaveCalled bool
	gotOld, gotFresh    *bookmark.Bookmark
}

func (gm *mockManager) IsEnabled() bool                                        { return gm.isEnabled }
func (gm *mockManager) Drop(ctx context.Context, gr *git.Repo) error           { return gm.dropErr }
func (gm *mockManager) Repos() []string                                        { return []string{} }
func (gm *mockManager) NewRepo(name string, opts ...git.RepoOptFunc) *git.Repo { return nil }
func (gm *mockManager) SaveChanges(ctx context.Context, gr *git.Repo, msg string) error {
	gm.savedMsg = msg
	return gm.saveChangesErr
}

func (gm *mockManager) Untrack(ctx context.Context, gr *git.Repo, msg string) error {
	gm.untrackMsg = msg
	return gm.untrackErr
}

func (gm *mockManager) IsTracked(name string) bool {
	gm.trackedName = name
	return gm.isTracked
}

func (gm *mockManager) UpdateAndSave(ctx context.Context, gr *git.Repo, old, fresh *bookmark.Bookmark, postRm git.PostRemovalFunc) error {
	gm.updateAndSaveCalled = true
	gm.gotOld = old
	gm.gotFresh = fresh
	return gm.updateAndSaveErr
}

func TestAdd(t *testing.T) {
	t.Parallel()

	errAdd := errors.New("add error")
	errSave := errors.New("save error")

	dummyBookmark := &bookmark.Bookmark{}

	tests := []struct {
		name         string
		enabled      bool
		tracked      bool
		writerErr    error
		saveErr      error
		bookmark     *bookmark.Bookmark
		wantSavedMsg bool
		wantErr      error
	}{
		{
			name:      "manager_disabled",
			enabled:   false,
			tracked:   true,
			writerErr: nil,
			saveErr:   nil,
			bookmark:  dummyBookmark,
			wantErr:   nil,
		},
		{
			name:      "repo_not_tracked",
			enabled:   true,
			tracked:   false,
			writerErr: nil,
			saveErr:   nil,
			bookmark:  dummyBookmark,
			wantErr:   nil,
		},
		{
			name:      "repo_add_error",
			enabled:   true,
			tracked:   true,
			writerErr: errAdd,
			saveErr:   nil,
			bookmark:  dummyBookmark,
			wantErr:   errAdd,
		},
		{
			name:      "save_changes_error",
			enabled:   true,
			tracked:   true,
			writerErr: nil,
			saveErr:   errSave,
			bookmark:  dummyBookmark,
			wantErr:   errSave,
		},
		{
			name:         "normal_success",
			enabled:      true,
			tracked:      true,
			writerErr:    nil,
			saveErr:      nil,
			bookmark:     dummyBookmark,
			wantSavedMsg: true,
			wantErr:      nil,
		},
		{
			name:         "nil_bookmark_success",
			enabled:      true,
			tracked:      true,
			writerErr:    nil,
			saveErr:      nil,
			bookmark:     nil,
			wantSavedMsg: true,
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := &mockManager{
				isEnabled:      tt.enabled,
				isTracked:      tt.tracked,
				saveChangesErr: tt.saveErr,
			}

			writerFunc := func(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
				return tt.writerErr
			}

			repo := git.NewRepo(tt.name, t.TempDir(), git.WithRepoWriter(writerFunc))

			err := Add(t.Context(), mgr, repo, tt.bookmark)

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

			if tt.wantSavedMsg {
				want := fmt.Sprintf("[%s] bookmark added", tt.name)
				if want != mgr.savedMsg {
					t.Fatalf("Add() savedMsg = %q; want %q", mgr.savedMsg, want)
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	errRm := errors.New("remove error")
	errSave := errors.New("save error")

	b1 := &bookmark.Bookmark{URL: "https://example.com/1"}
	b2 := &bookmark.Bookmark{URL: "https://example.com/2"}

	tests := []struct {
		name         string
		enabled      bool
		tracked      bool
		removerErr   error
		saveErr      error
		bookmarks    []*bookmark.Bookmark
		wantSavedMsg string
		wantErr      error
	}{
		{
			name:       "manager_disabled",
			enabled:    false,
			tracked:    true,
			removerErr: nil,
			saveErr:    nil,
			bookmarks:  []*bookmark.Bookmark{b1},
			wantErr:    nil,
		},
		{
			name:       "repo_not_tracked",
			enabled:    true,
			tracked:    false,
			removerErr: nil,
			saveErr:    nil,
			bookmarks:  []*bookmark.Bookmark{b1},
			wantErr:    nil,
		},
		{
			name:       "repo_rm_many_error",
			enabled:    true,
			tracked:    true,
			removerErr: errRm,
			saveErr:    nil,
			bookmarks:  []*bookmark.Bookmark{b1},
			wantErr:    errRm,
		},
		{
			name:       "save_changes_error",
			enabled:    true,
			tracked:    true,
			removerErr: nil,
			saveErr:    errSave,
			bookmarks:  []*bookmark.Bookmark{b1},
			wantErr:    errSave,
		},
		{
			name:         "normal_success_multiple_bookmarks",
			enabled:      true,
			tracked:      true,
			removerErr:   nil,
			saveErr:      nil,
			bookmarks:    []*bookmark.Bookmark{b1, b2},
			wantSavedMsg: "[test-repo] remove bookmarks",
			wantErr:      nil,
		},
		{
			name:         "empty_bookmarks_slice",
			enabled:      true,
			tracked:      true,
			removerErr:   nil,
			saveErr:      nil,
			bookmarks:    []*bookmark.Bookmark{},
			wantSavedMsg: "[test-repo] remove bookmarks",
			wantErr:      nil,
		},
		{
			name:         "nil_bookmarks_slice",
			enabled:      true,
			tracked:      true,
			removerErr:   nil,
			saveErr:      nil,
			bookmarks:    nil,
			wantSavedMsg: "[test-repo] remove bookmarks",
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := &mockManager{
				isEnabled:      tt.enabled,
				isTracked:      tt.tracked,
				saveChangesErr: tt.saveErr,
			}

			removerFunc := func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
				return tt.removerErr
			}

			repo := git.NewRepo("test-repo", t.TempDir(), git.WithRepoRemover(removerFunc))

			err := Remove(t.Context(), mgr, repo, tt.bookmarks)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Remove() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Remove() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Remove() unexpected error: %v", err)
			}

			if tt.wantSavedMsg != "" && mgr.savedMsg != tt.wantSavedMsg {
				t.Fatalf("Remove() savedMsg = %q; want %q", mgr.savedMsg, tt.wantSavedMsg)
			}
		})
	}
}

type mockConsole struct {
	confirmDrop    bool
	confirmUntrack bool
	printErr       error
	printedMsg     string
}

func (c *mockConsole) Confirm(ctx context.Context, prompt, defaultAns string) bool {
	if strings.Contains(prompt, "drop git repo") {
		return c.confirmDrop
	}
	if strings.Contains(prompt, "untrack database") {
		return c.confirmUntrack
	}
	return false
}

func (c *mockConsole) SuccessMesg(a ...any) string { return fmt.Sprintf("Successfully: %s", a[0]) }
func (c *mockConsole) Print(ctx context.Context, msg string) error {
	c.printedMsg = msg
	return c.printErr
}

func TestDrop(t *testing.T) {
	t.Parallel()

	errDrop := errors.New("drop failed")
	errUntrack := errors.New("untrack failed")
	errPrint := errors.New("print failed")

	tests := []struct {
		name           string
		enabled        bool
		tracked        bool
		confirmDrop    bool
		confirmUntrack bool
		dropErr        error
		untrackErr     error
		printErr       error
		wantUntrackMsg string
		wantPrintedMsg string
		wantErr        error
	}{
		{
			name:           "manager_disabled",
			enabled:        false,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: true,
			wantErr:        nil,
		},
		{
			name:           "repo_not_tracked",
			enabled:        true,
			tracked:        false,
			confirmDrop:    true,
			confirmUntrack: true,
			wantErr:        nil,
		},
		{
			name:           "decline_first_confirm",
			enabled:        true,
			tracked:        true,
			confirmDrop:    false,
			confirmUntrack: true,
			wantErr:        nil,
		},
		{
			name:           "drop_error",
			enabled:        true,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: true,
			dropErr:        errDrop,
			wantErr:        errDrop,
		},
		{
			name:           "decline_second_confirm",
			enabled:        true,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: false,
			wantErr:        nil,
		},
		{
			name:           "untrack_error",
			enabled:        true,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: true,
			untrackErr:     errUntrack,
			wantErr:        errUntrack,
		},
		{
			name:           "print_error",
			enabled:        true,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: true,
			printErr:       errPrint,
			wantUntrackMsg: "[test-repo] remove tracking",
			wantErr:        errPrint,
		},
		{
			name:           "full_success",
			enabled:        true,
			tracked:        true,
			confirmDrop:    true,
			confirmUntrack: true,
			wantUntrackMsg: "[test-repo] remove tracking",
			wantPrintedMsg: "Successfully: database untracked\n",
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := &mockManager{
				isEnabled:  tt.enabled,
				isTracked:  tt.tracked,
				dropErr:    tt.dropErr,
				untrackErr: tt.untrackErr,
			}

			cons := &mockConsole{
				confirmDrop:    tt.confirmDrop,
				confirmUntrack: tt.confirmUntrack,
				printErr:       tt.printErr,
			}

			repo := git.NewRepo("test-repo", t.TempDir())

			err := Drop(t.Context(), mgr, repo, cons)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Drop() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Drop() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Drop() unexpected error: %v", err)
			}

			if tt.wantUntrackMsg != "" && mgr.untrackMsg != tt.wantUntrackMsg {
				t.Fatalf("Drop() untrackMsg = %q; want %q", mgr.untrackMsg, tt.wantUntrackMsg)
			}

			if tt.wantPrintedMsg != "" && cons.printedMsg != tt.wantPrintedMsg {
				t.Fatalf("Drop() printedMsg = %q; want %q", cons.printedMsg, tt.wantPrintedMsg)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	errUpdateAndSave := errors.New("update and save failed")

	old := &bookmark.Bookmark{ID: 1, URL: "https://old.com"}
	fresh := &bookmark.Bookmark{ID: 1, URL: "https://fresh.com"}

	tests := []struct {
		name                    string
		isEnabled               bool
		isTracked               bool
		repoName                string
		updateAndSaveErr        error
		old, fresh              *bookmark.Bookmark
		want                    error
		wantUpdateAndSaveCalled bool
	}{
		{
			name:                    "enabled_and_tracked_calls_update_and_save",
			isEnabled:               true,
			isTracked:               true,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: true,
		},
		{
			name:                    "disabled_manager_skips_update",
			isEnabled:               false,
			isTracked:               true,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: false,
		},
		{
			name:                    "untracked_repo_skips_update",
			isEnabled:               true,
			isTracked:               false,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: false,
		},
		{
			name:                    "disabled_and_untracked_skips_update",
			isEnabled:               false,
			isTracked:               false,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: false,
		},
		{
			name:                    "update_and_save_fails",
			isEnabled:               true,
			isTracked:               true,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   fresh,
			updateAndSaveErr:        errUpdateAndSave,
			want:                    errUpdateAndSave,
			wantUpdateAndSaveCalled: true,
		},
		{
			name:                    "empty_repo_name_still_checked",
			isEnabled:               true,
			isTracked:               false,
			repoName:                "",
			old:                     old,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: false,
		},
		{
			name:                    "nil_old_bookmark_passed_through",
			isEnabled:               true,
			isTracked:               true,
			repoName:                "myrepo",
			old:                     nil,
			fresh:                   fresh,
			wantUpdateAndSaveCalled: true,
		},
		{
			name:                    "nil_fresh_bookmark_passed_through",
			isEnabled:               true,
			isTracked:               true,
			repoName:                "myrepo",
			old:                     old,
			fresh:                   nil,
			wantUpdateAndSaveCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fm := &mockManager{
				isEnabled:        tt.isEnabled,
				isTracked:        tt.isTracked,
				updateAndSaveErr: tt.updateAndSaveErr,
			}

			gr := git.NewRepo(tt.repoName, t.TempDir())

			err := Update(t.Context(), fm, gr, tt.old, tt.fresh)

			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("Update() error = %v, want %v", err, tt.want)
				}
			} else if err != nil {
				t.Fatalf("Update() unexpected error: %v", err)
			}

			if fm.updateAndSaveCalled != tt.wantUpdateAndSaveCalled {
				t.Errorf("UpdateAndSave called = %v, want %v", fm.updateAndSaveCalled, tt.wantUpdateAndSaveCalled)
			}

			if tt.wantUpdateAndSaveCalled {
				if fm.trackedName != tt.repoName {
					t.Errorf("IsTracked called with %q, want %q", fm.trackedName, tt.repoName)
				}
				if fm.gotOld != tt.old {
					t.Errorf("UpdateAndSave got old = %v, want %v (same pointer)", fm.gotOld, tt.old)
				}
				if fm.gotFresh != tt.fresh {
					t.Errorf("UpdateAndSave got fresh = %v, want %v (same pointer)", fm.gotFresh, tt.fresh)
				}
			}
		})
	}
}
