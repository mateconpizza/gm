package bookio

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestFileWriter(t *testing.T) {
	t.Parallel()

	errWriteFailed := errors.New("write failed")

	tests := []struct {
		name        string
		bookmarks   []*bookmark.Bookmark
		failOn      map[string]error // bookmark.URL -> error to return
		ctxCanceled bool
		wantCount   uint32
		wantErr     error
	}{
		{
			name:      "normal_multiple_bookmarks",
			bookmarks: testutil.NewBookmarkSlice(t, 3),
			wantCount: 3,
		},
		{
			name:      "single_bookmark",
			bookmarks: testutil.NewBookmarkSlice(t, 1),
			wantCount: 1,
		},
		{
			name:      "empty_bookmarks",
			bookmarks: []*bookmark.Bookmark{},
			wantCount: 0,
		},
		{
			name:      "nil_bookmarks",
			bookmarks: nil,
			wantCount: 0,
		},
		{
			name: "one_write_fails",
			bookmarks: []*bookmark.Bookmark{
				{URL: "a.json"}, {URL: "bad.json"}, {URL: "c.json"},
			},
			failOn:  map[string]error{"bad.json": errWriteFailed},
			wantErr: errWriteFailed,
			// wantCount intentionally not asserted exactly here: the two
			// successful writes may or may not have completed depending on
			// goroutine scheduling, so this case only asserts the error.
		},
		{
			name: "all_writes_fail",
			bookmarks: []*bookmark.Bookmark{
				{URL: "bad1.json"}, {URL: "bad2.json"},
			},
			failOn: map[string]error{
				"bad1.json": errWriteFailed,
				"bad2.json": errWriteFailed,
			},
			wantCount: 0,
			wantErr:   errWriteFailed,
		},
		{
			name:        "context_canceled_before_write",
			bookmarks:   []*bookmark.Bookmark{{URL: "a.json"}, {URL: "b.json"}},
			ctxCanceled: true,
			wantCount:   0,
			wantErr:     context.Canceled,
		},
		{
			name:      "many_bookmarks_boundary",
			bookmarks: testutil.NewBookmarkSlice(t, 50),
			wantCount: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			written := make(map[string]bool)

			writer := func(_ context.Context, b *bookmark.Bookmark) error {
				if tt.failOn != nil {
					if err, ok := tt.failOn[b.URL]; ok {
						return err
					}
				}
				mu.Lock()
				written[b.URL] = true
				mu.Unlock()
				return nil
			}

			f := NewFileWriter(writer)

			ctx := t.Context()
			if tt.ctxCanceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			for _, b := range tt.bookmarks {
				f.Write(ctx, b)
			}

			err := f.Wait()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Wait() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Wait() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				if tt.name != "one_write_fails" {
					// skip exact count assertion for the partial-failure case;
					// assert it everywhere else, including all-fail and
					// canceled-context cases.
					if got := f.Current(); got != tt.wantCount {
						t.Fatalf("Current() = %d; want %d", got, tt.wantCount)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Wait() unexpected error: %v", err)
			}

			if got := f.Current(); got != tt.wantCount {
				t.Fatalf("Current() = %d; want %d", got, tt.wantCount)
			}

			mu.Lock()
			gotWritten := len(written)
			mu.Unlock()

			if uint32(gotWritten) != tt.wantCount {
				t.Fatalf("writer was called for %d bookmarks; want %d", gotWritten, tt.wantCount)
			}
		})
	}
}
