package dbops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/ansi"
)

func TestSelector_Select(t *testing.T) {
	t.Parallel()

	newTestFile := func(t *testing.T, dir, name string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("failed to create dirs for test file: %v", err)
		}
		if err := os.WriteFile(path, []byte("test data"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
		return path
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T) *Selector
		want       []string
		wantErr    error
		expectErr  bool
		wantErrMsg string
	}{
		{
			name: "normal_selection",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				tempDir := t.TempDir()
				newTestFile(t, tempDir, "data1.db")
				targetPath := newTestFile(t, tempDir, "data2.db")
				app := testutil.NewApp(t).WithHomePath(tempDir)

				r := testutil.NewMenuRunner().
					WithRetCode(0).
					WithOutput(targetPath)

				return NewSelector(app, app.Path.Home()).
					WithOpts(menu.WithRunner(r))
			},
			want:    []string{"data2.db"},
			wantErr: nil,
		},
		{
			name: "zero_items_empty_directory",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				return NewSelector(nil, t.TempDir())
			},
			want:    nil,
			wantErr: ErrNoItems,
		},
		{
			name: "zero_items_wrong_extension",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				tempDir := t.TempDir()

				// No .db files
				newTestFile(t, tempDir, "data.txt")
				newTestFile(t, tempDir, "notes.md")

				app := testutil.NewApp(t).
					WithHomePath(tempDir)

				return NewSelector(app, app.Path.Home())
			},
			want:    nil,
			wantErr: ErrNoItems,
		},
		{
			name: "boundary_all_items_excluded",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				dir := t.TempDir()

				skip1 := newTestFile(t, dir, "skip1.db")
				skip2 := newTestFile(t, dir, "skip2.db")
				app := testutil.NewApp(t).
					WithHomePath(dir)

				r := testutil.NewMenuRunner().
					WithRetCode(0).
					WithOutput("")

				return NewSelector(app, app.Path.Home()).
					WithFilter(func(s string) bool {
						return s != skip1 && s != skip2
					}).
					WithOpts(menu.WithRunner(r))
			},
			want:    nil,
			wantErr: ErrNoItems,
		},
		{
			name: "boundary_all_items_filtered_out",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				tempDir := t.TempDir()
				newTestFile(t, tempDir, "match.db")
				newTestFile(t, tempDir, "other.db")
				app := testutil.NewApp(t).
					WithHomePath(tempDir)

				r := testutil.NewMenuRunner().
					WithRetCode(0).
					WithOutput("")

				return NewSelector(app, app.Path.Home()).
					WithFilter(func(path string) bool { return false }). // filter rejects everything
					WithOpts(menu.WithRunner(r))
			},
			want:    nil,
			wantErr: ErrNoItems,
		},
		{
			name: "error_action_aborted",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				tempDir := t.TempDir()
				newTestFile(t, tempDir, "data.db")
				app := testutil.NewApp(t).WithHomePath(tempDir)

				r := testutil.NewMenuRunner().
					WithRetCode(130)

				return NewSelector(app, app.Path.Home()).
					WithOpts(menu.WithRunner(r))
			},
			want:    nil,
			wantErr: sys.ErrActionAborted,
		},
		{
			name: "custom_formatters_applied",
			setup: func(t *testing.T) *Selector {
				t.Helper()
				tempDir := t.TempDir()
				newTestFile(t, tempDir, "custom.db")
				app := testutil.NewApp(t)

				targetPath := filepath.Join(tempDir, "custom.db")
				formattedOutput := "WRAP_FMT_" + targetPath

				itemFmt := func(ctx context.Context, p *ansi.Palette, path string, maxWidth int) string {
					return "FMT_" + path
				}

				customFmt := func(in string) string {
					return "WRAP_" + in
				}

				r := testutil.NewMenuRunner().
					WithOutput(formattedOutput)

				return NewSelector(app, tempDir).
					WithItemFormatter(itemFmt).
					WithItemDecorator(customFmt).
					WithOpts(menu.WithRunner(r))
			},
			want:    []string{"custom.db"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.setup(t)
			pathList, err := s.Select(t.Context())

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Select() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Select() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.expectErr {
				if err == nil {
					t.Fatalf("Select() expected an error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Select() expected error containing %q, got %v", tt.wantErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Select() unexpected error: %v", err)
			}

			got := make([]string, 0, len(pathList))
			for i := range pathList {
				got = append(got, filepath.Base(pathList[i]))
			}

			t.Log(got)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Select() = %v; want %v", pathList, tt.want)
			}
		})
	}
}
