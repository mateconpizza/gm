package port

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type fakeStoreReader struct {
	bookmarks []*bookmark.Bookmark
	err       error
	name      string
}

func (f *fakeStoreReader) All(ctx context.Context) ([]*bookmark.Bookmark, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bookmarks, nil
}

func (f *fakeStoreReader) Name() string { return f.name }
func (f *fakeStoreReader) Close()       {}

type fakeSelector[T any] struct {
	items    []T
	selected []T
	err      error
}

func (f *fakeSelector[T]) Select(items []T) ([]T, error) {
	f.items = items

	if f.err != nil {
		return nil, f.err
	}

	return f.selected, nil
}

func Test_ImportBookmarksFromBackup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		srcBookmarks []*bookmark.Bookmark
		srcErr       error
		selected     []*bookmark.Bookmark
		selectErr    error
		promptInput  string // stdin fed to the interactive confirm prompt in importPipeline; "" if never reached
		wantErr      error
		wantErrAny   bool
	}{
		{
			name:         "normal_import_with_selected_bookmarks",
			srcBookmarks: testutil.NewBookmarkSlice(t, 3),
			selected:     testutil.NewBookmarkSlice(t, 3),
			promptInput:  "y\n",
		},
		{
			name:         "empty_selection_returns_exit_failure",
			srcBookmarks: testutil.NewBookmarkSlice(t, 3),
			selected:     []*bookmark.Bookmark{},
			wantErr:      sys.ErrExitFailure,
			// no promptInput needed: short-circuits before the prompt
		},
		{
			name:         "single_bookmark_boundary",
			srcBookmarks: []*bookmark.Bookmark{testutil.NewBookmark(t)},
			selected:     []*bookmark.Bookmark{testutil.NewBookmark(t)},
			promptInput:  "y\n",
		},
		{
			name:       "srcDB_all_returns_error",
			srcErr:     errors.New("db read failure"),
			wantErrAny: true,
		},
		{
			name:         "selector_returns_error",
			srcBookmarks: testutil.NewBookmarkSlice(t, 2),
			selectErr:    errors.New("selection cancelled"),
			wantErrAny:   true,
		},
		{
			name:         "no_bookmarks_in_source_empty_selection",
			srcBookmarks: []*bookmark.Bookmark{},
			selected:     []*bookmark.Bookmark{},
			wantErr:      sys.ErrExitFailure,
		},
		{
			name:         "large_number_of_bookmarks",
			srcBookmarks: testutil.NewBookmarkSlice(t, 50),
			selected:     testutil.NewBookmarkSlice(t, 50),
			promptInput:  "y\n",
		},
		{
			name:         "user_declines_import_prompt",
			srcBookmarks: testutil.NewBookmarkSlice(t, 2),
			selected:     testutil.NewBookmarkSlice(t, 2),
			promptInput:  "n\n",
			wantErrAny:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			d := testutil.NewDeps(t)

			app, err := d.Application(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
			d.SetRepo(r)

			if tt.promptInput != "" {
				d.Console().SetReader(strings.NewReader(tt.promptInput))
			}

			src := &fakeStoreReader{
				bookmarks: tt.srcBookmarks,
				err:       tt.srcErr,
				name:      "backup.db",
			}
			m := &fakeSelector[*bookmark.Bookmark]{
				selected: tt.selected,
				err:      tt.selectErr,
			}

			err = importBookmarksFromBackup(ctx, d, src, m)

			if tt.wantErrAny {
				if err == nil {
					t.Fatalf("importBookmarksFromBackup() expected an error, got nil")
				}
				return
			}

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("importBookmarksFromBackup() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("importBookmarksFromBackup() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("importBookmarksFromBackup() unexpected error: %v", err)
			}
		})
	}
}
