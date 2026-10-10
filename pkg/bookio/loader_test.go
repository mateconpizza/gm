package bookio

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestFileLoaderTwo(t *testing.T) {
	t.Parallel()

	errLoadFailed := errors.New("load failed")

	tests := []struct {
		name        string
		paths       []string
		failOn      map[string]error // paths that should fail when loaded
		ctxCanceled bool
		want        []string // expected URLs in result (order-independent)
		wantErr     error
	}{
		{
			name:  "normal_multiple_paths",
			paths: []string{"a.json", "b.json", "c.json"},
			want:  []string{"a.json", "b.json", "c.json"},
		},
		{
			name:  "single_path",
			paths: []string{"only.json"},
			want:  []string{"only.json"},
		},
		{
			name:  "empty_paths",
			paths: []string{},
			want:  []string{},
		},
		{
			name:  "nil_paths",
			paths: nil,
			want:  []string{},
		},
		{
			name:    "one_path_fails",
			paths:   []string{"a.json", "bad.json", "c.json"},
			failOn:  map[string]error{"bad.json": errLoadFailed},
			want:    nil,
			wantErr: errLoadFailed,
		},
		{
			name:    "all_paths_fail",
			paths:   []string{"bad1.json", "bad2.json"},
			failOn:  map[string]error{"bad1.json": errLoadFailed, "bad2.json": errLoadFailed},
			want:    nil,
			wantErr: errLoadFailed,
		},
		{
			name:        "context_canceled_before_load",
			paths:       []string{"a.json", "b.json"},
			ctxCanceled: true,
			want:        nil,
			wantErr:     context.Canceled,
		},
		{
			name:  "many_paths_boundary",
			paths: genPaths(50),
			want:  genPaths(50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loader := func(_ context.Context, path string) (*bookmark.Bookmark, error) {
				if tt.failOn != nil {
					if err, ok := tt.failOn[path]; ok {
						return nil, err
					}
				}
				return &bookmark.Bookmark{URL: path}, nil
			}

			f := NewFileLoader(loader)

			ctx := context.Background()
			if tt.ctxCanceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			for _, p := range tt.paths {
				f.Load(ctx, p)
			}

			got, err := f.Results()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Results() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Results() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Results() unexpected error: %v", err)
			}

			gotURLs := make([]string, 0, len(got))
			for _, b := range got {
				gotURLs = append(gotURLs, b.URL)
			}
			slices.Sort(gotURLs)

			want := slices.Clone(tt.want)
			slices.Sort(want)

			if !slices.Equal(gotURLs, want) {
				t.Fatalf("Results() URLs = %v; want %v", gotURLs, want)
			}
		})
	}
}

func genPaths(n int) []string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = "file" + string(rune('0'+i%10)) + ".json"
	}
	return paths
}
