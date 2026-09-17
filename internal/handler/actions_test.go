package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var errMockOpener = errors.New("mock opener error")

type mockSpinner struct {
	started bool
	done    bool
}

func (m *mockSpinner) Start(ctx context.Context) { m.started = true }
func (m *mockSpinner) Done(mesg ...string)       { m.done = true }

func TestOpenInBrowser(t *testing.T) {
	t.Parallel()

	bs := testutil.NewBookmarkSlice(t, 100)

	tests := []struct {
		name       string
		bookmarks  []*bookmark.Bookmark
		openerFunc func(ctx context.Context, s string) error
		wantErr    error
	}{
		{
			name: "normal_success",
			bookmarks: []*bookmark.Bookmark{
				{URL: "https://example.com/1"},
				{URL: "https://example.com/2"},
			},
			openerFunc: func(ctx context.Context, s string) error { return nil },
			wantErr:    nil,
		},
		{
			name:       "empty_bookmarks",
			bookmarks:  []*bookmark.Bookmark{},
			openerFunc: func(ctx context.Context, s string) error { return errors.New("should not be called") },
			wantErr:    nil,
		},
		{
			name:       "nil_bookmarks_slice",
			bookmarks:  nil,
			openerFunc: func(ctx context.Context, s string) error { return errors.New("should not be called") },
			wantErr:    nil,
		},
		{
			name: "opener_error",
			bookmarks: []*bookmark.Bookmark{
				{URL: "https://example.com/fail"},
			},
			openerFunc: func(ctx context.Context, s string) error { return errMockOpener },
			wantErr:    errMockOpener,
		},
		{
			name: "context_already_canceled",
			bookmarks: []*bookmark.Bookmark{
				{URL: "https://example.com/cancel"},
			},
			openerFunc: func(ctx context.Context, s string) error { return errors.New("should not be called") },
			wantErr:    context.Canceled,
		},
		{
			name: "partial_failure",
			bookmarks: []*bookmark.Bookmark{
				{URL: "https://example.com/good"},
				{URL: "https://example.com/fail"},
				{URL: "https://example.com/good2"},
			},
			openerFunc: func(ctx context.Context, s string) error {
				if s == "https://example.com/fail" {
					return errMockOpener
				}
				return nil
			},
			wantErr: errMockOpener,
		},
		{
			name: "empty_url_string",
			bookmarks: []*bookmark.Bookmark{
				{URL: ""},
			},
			openerFunc: func(ctx context.Context, s string) error {
				if s == "" {
					return errMockOpener
				}
				return nil
			},
			wantErr: errMockOpener,
		},
		{
			name:       "large_number_of_bookmarks",
			bookmarks:  bs,
			openerFunc: func(ctx context.Context, s string) error { return nil },
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spin := &mockSpinner{}
			opts := &openOpts{
				spinner:   spin,
				bookmarks: tt.bookmarks,
				opener:    tt.openerFunc,
			}

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(t.Context())
				cancel()
			}

			err := openInBrowser(ctx, opts)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("openInBrowser() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("openInBrowser() expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("openInBrowser() unexpected error: %v", err)
			}

			if !spin.started {
				t.Errorf("expected spinner.Start() to be called")
			}
			if !spin.done {
				t.Errorf("expected spinner.Done() to be called")
			}
		})
	}
}
