//go:build ignore
package gitops

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type fakeSpinner struct{}

func (sp fakeSpinner) Start(ctx context.Context)                       {}
func (sp fakeSpinner) Done(mesg ...string)                             {}
func (sp fakeSpinner) Fail(mesg ...string)                             {}
func (sp fakeSpinner) AddPrefixDecorator(fn func(mesg string) string)  {}
func (sp fakeSpinner) SetMessageDecorator(fn func(mesg string) string) {}
func (sp fakeSpinner) UpdateMesg(s string)                             {}
func (sp fakeSpinner) UpdatePrefix(s string)                           {}

func TestReadJSONRepo(t *testing.T) {
	t.Parallel()

	errSimulatedLoader := errors.New("simulated loader error")

	tests := []struct {
		name        string
		files       []string
		invalidRoot bool
		cancelCtx   bool
		loaderFunc  func(ctx context.Context, path string) (*bookmark.Bookmark, error)
		filterFunc  func(path string, d fs.DirEntry) bool
		want        []*bookmark.Bookmark
		wantErr     error
	}{
		{
			name:  "normal_typical_inputs",
			files: []string{"a.json", "b.json"},
			loaderFunc: func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
				return &bookmark.Bookmark{Title: filepath.Base(path)}, nil
			},
			filterFunc: func(path string, d fs.DirEntry) bool { return !d.IsDir() },
			want: []*bookmark.Bookmark{
				{Title: "a.json"},
				{Title: "b.json"},
			},
		},
		{
			name:  "empty_directory_zero_files",
			files: []string{},
			loaderFunc: func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
				return &bookmark.Bookmark{Title: filepath.Base(path)}, nil
			},
			filterFunc: func(path string, d fs.DirEntry) bool { return !d.IsDir() },
			want:       []*bookmark.Bookmark{},
		},
		{
			name:  "boundary_file_filtering",
			files: []string{"a.json", "ignore.txt"},
			loaderFunc: func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
				return &bookmark.Bookmark{Title: filepath.Base(path)}, nil
			},
			filterFunc: func(path string, d fs.DirEntry) bool {
				return !d.IsDir() && filepath.Ext(path) == ".json"
			},
			want: []*bookmark.Bookmark{
				{Title: "a.json"},
			},
		},
		{
			name:        "error_invalid_root_path",
			invalidRoot: true,
			wantErr:     fs.ErrNotExist,
		},
		{
			name:       "error_context_cancelled",
			files:      []string{"a.json"},
			cancelCtx:  true,
			filterFunc: func(path string, d fs.DirEntry) bool { return !d.IsDir() },
			wantErr:    context.Canceled,
		},
		{
			name:  "error_loader_failure",
			files: []string{"a.json"},
			loaderFunc: func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
				return nil, errSimulatedLoader
			},
			filterFunc: func(path string, d fs.DirEntry) bool { return !d.IsDir() },
			wantErr:    errSimulatedLoader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			root := t.TempDir()
			if tt.invalidRoot {
				root = filepath.Join(root, "nonexistent_dir")
			} else {
				for _, name := range tt.files {
					testutil.NewFile(t, root, name, []byte("x"))
				}
			}

			loaderFn := tt.loaderFunc
			if loaderFn == nil {
				loaderFn = func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
					return &bookmark.Bookmark{}, nil
				}
			}
			filterFn := tt.filterFunc
			if filterFn == nil {
				filterFn = func(path string, d fs.DirEntry) bool { return !d.IsDir() }
			}

			cfg := &readerCfg{
				root:     root,
				repoPath: root,
				total:    len(tt.files),
				spinner:  &fakeSpinner{},
				loader: &bookio.RepositoryLoader{
					Func:       loaderFn,
					FileFilter: filterFn,
					Prefix:     "test_prefix",
				},
			}

			got, err := newFileRepoReader(ctx, cfg)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ReadJSONRepo() expected error want: %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadJSONRepo() unexpected error: %v", err)
			}

			// results from the async loader are not guaranteed to be ordered.
			// we sort them by title before asserting equality.
			sort.Slice(got, func(i, j int) bool {
				return got[i].Title < got[j].Title
			})

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadJSONRepo() = %v; want %v", got, tt.want)
			}
		})
	}
}
