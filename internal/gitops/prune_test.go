package gitops

import (
	"context"
	"errors"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

var _ gitRepo = (*mockGitRepo)(nil)

type mockGitRepo struct {
	readErr      error
	bookmarks    []*bookmark.Bookmark
	name         string
	addErr       error
	rmManyErr    error
	addedBooks   []*bookmark.Bookmark
	removedBooks []*bookmark.Bookmark
}

func (m *mockGitRepo) Name() string                    { return m.name }
func (m *mockGitRepo) Read(ctx context.Context) error  { return m.readErr }
func (m *mockGitRepo) Bookmarks() []*bookmark.Bookmark { return m.bookmarks }
func (m *mockGitRepo) Add(ctx context.Context, bs []*bookmark.Bookmark) error {
	m.addedBooks = bs
	m.bookmarks = append(m.bookmarks, bs...)
	return m.addErr
}

func (m *mockGitRepo) RmMany(ctx context.Context, bs []*bookmark.Bookmark, postRm git.PostRemovalFunc) error {
	m.removedBooks = bs
	return m.rmManyErr
}

func TestRepoReconcilerReconcile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dbBookmarks []*bookmark.Bookmark
		repoSetup   func(*mockGitRepo)
		saveFn      saveChangesFunc
		wantErr     error
	}{
		{
			name:        "empty_bookmarks",
			dbBookmarks: []*bookmark.Bookmark{},
			repoSetup:   func(m *mockGitRepo) {},
			saveFn:      func(ctx context.Context, msg string) error { return nil },
			wantErr:     bookmark.ErrBookmarkNotFound,
		},
		{
			name: "normal_success",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "bookmark1"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{}
				m.addErr = nil
				m.rmManyErr = nil
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: git.ErrGitUpToDate,
		},
		{
			name: "single_bookmark_boundary",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "single"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{}
				m.addErr = nil
				m.rmManyErr = nil
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: git.ErrGitUpToDate,
		},
		{
			name: "repo_read_error",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "bookmark1"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = errors.New("read failed")
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: errors.New("read failed"),
		},
		{
			name: "add_missing_error",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "bookmark1"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{}
				m.addErr = errors.New("add failed")
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: errors.New("add failed"),
		},
		{
			name: "prune_stale_error",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "bookmark1"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{
					{ID: 2, Title: "stale"},
				}
				m.addErr = nil
				m.rmManyErr = errors.New("remove failed")
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: errors.New("remove failed"),
		},
		{
			name: "multiple_bookmarks_success",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "book1"},
				{ID: 2, Title: "book2"},
				{ID: 3, Title: "book3"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{
					{ID: 2, Title: "book2"},
				}
				m.addErr = nil
				m.rmManyErr = nil
			},
			saveFn:  func(ctx context.Context, msg string) error { return nil },
			wantErr: git.ErrGitUpToDate,
		},
		{
			name: "save_changes_error",
			dbBookmarks: []*bookmark.Bookmark{
				{ID: 1, Title: "bookmark1"},
			},
			repoSetup: func(m *mockGitRepo) {
				m.readErr = nil
				m.bookmarks = []*bookmark.Bookmark{}
				m.addErr = nil
				m.rmManyErr = nil
			},
			saveFn:  func(ctx context.Context, msg string) error { return errors.New("save failed") },
			wantErr: errors.New("save failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &mockGitRepo{name: "test-repo"}
			tt.repoSetup(repo)

			for i := range tt.dbBookmarks {
				tt.dbBookmarks[i].GenChecksum()
			}

			reconciler := newRepoReconciler(repo, tt.dbBookmarks, tt.saveFn)
			err := reconciler.Reconcile(context.Background())

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Reconcile() expected: error %v, got: nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Fatalf("Reconcile() expected: error %v, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Reconcile() unexpected error: %v", err)
			}
		})
	}
}
