package gitops

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/mateconpizza/gm/internal/locker/gpg"
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

			cfg := &RepoReaderCfg{
				root:    root,
				total:   len(tt.files),
				spinner: &fakeSpinner{},
				loader: &bookio.RepositoryLoader{
					Func:       loaderFn,
					FileFilter: filterFn,
					Prefix:     "test_prefix",
				},
			}

			got, err := ReadJSONRepo(ctx, cfg)

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

func TestReadGPGRepo(t *testing.T) {
	t.Parallel()

	errFakePrompt := errors.New("err fake prompt")
	errFakeLoader := errors.New("err fake loader")

	type fileSpec struct {
		name string
		data []byte
	}

	tests := []struct {
		name string

		files       []fileSpec
		matchExt    string
		promptErr   error
		loaderErr   error // returned by loader.Func for every matched file
		missingRoot bool  // root path does not exist, forcing a walk error

		wantCount int
		wantErr   error
	}{
		{
			name:      "normal_multiple_matching_files",
			files:     []fileSpec{{"a.gpg", []byte("1")}, {"b.gpg", []byte("2")}, {"skip.txt", []byte("3")}},
			matchExt:  gpg.Extension,
			wantCount: 2,
			wantErr:   nil,
		},
		{
			name:      "no_matching_files",
			files:     []fileSpec{{"a.txt", []byte("1")}, {"b.txt", []byte("2")}},
			matchExt:  gpg.Extension,
			wantCount: 0,
			wantErr:   nil,
		},
		{
			name:      "empty_root_dir",
			files:     nil,
			matchExt:  gpg.Extension,
			wantCount: 0,
			wantErr:   nil,
		},
		{
			name:      "single_file_boundary",
			files:     []fileSpec{{"only.gpg", []byte("1")}},
			matchExt:  gpg.Extension,
			wantCount: 1,
			wantErr:   nil,
		},
		{
			name:        "missing_root_dir_walk_error",
			missingRoot: true,
			matchExt:    gpg.Extension,
			wantCount:   0,
			wantErr:     fs.ErrNotExist,
		},
		{
			name:      "passphrase_prompt_error",
			files:     []fileSpec{{"a.gpg", []byte("1")}},
			matchExt:  gpg.Extension,
			promptErr: errFakePrompt,
			wantCount: 0,
			wantErr:   errFakePrompt,
		},
		{
			name:      "loader_func_error",
			files:     []fileSpec{{"a.gpg", []byte("1")}},
			matchExt:  gpg.Extension,
			loaderErr: errFakeLoader,
			wantCount: 0,
			wantErr:   errFakeLoader,
		},
		{
			name:      "context_canceled_before_walk",
			files:     []fileSpec{{"a.gpg", []byte("1")}},
			matchExt:  gpg.Extension,
			wantCount: 0,
			wantErr:   context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()

			if tt.missingRoot {
				root = filepath.Join(root, "does-not-exist")
			} else {
				for _, fs := range tt.files {
					testutil.NewFile(t, root, fs.name, fs.data)
				}
			}

			var loaderCalls atomic.Int32

			loader := &bookio.RepositoryLoader{
				Func: func(_ context.Context, path string) (*bookmark.Bookmark, error) {
					loaderCalls.Add(1)
					if tt.loaderErr != nil {
						return nil, tt.loaderErr
					}
					return &bookmark.Bookmark{URL: path}, nil
				},
				Prefix: "%d/%d",
				FileFilter: func(path string, d fs.DirEntry) bool {
					if d.IsDir() || tt.matchExt == "" {
						return false
					}
					return filepath.Ext(path) == tt.matchExt
				},
			}

			sp := &fakeSpinner{}

			cfg := &RepoReaderCfg{
				root:    root,
				total:   len(tt.files),
				loader:  loader,
				spinner: sp,
				promptPassphrase: func(_ context.Context, _ *bookio.FileLoader, _ spinner, _ string, done *bool) error {
					if tt.promptErr != nil {
						return tt.promptErr
					}
					*done = true
					return nil
				},
			}

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			got, err := ReadGPGRepo(ctx, cfg)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ReadGPGRepo() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ReadGPGRepo() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadGPGRepo() unexpected error: %v", err)
			}

			if len(got) != tt.wantCount {
				t.Fatalf("ReadGPGRepo() returned %d bookmarks; want %d", len(got), tt.wantCount)
			}
		})
	}
}
