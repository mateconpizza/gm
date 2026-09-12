package git

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestRemoveAllExcept(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupFiles []string
		keep       map[string]struct{}
		useBadDir  bool
		wantRemain []string
		wantErr    error
	}{
		{
			name:       "normal_mixed",
			setupFiles: []string{"keep.txt", "remove1.txt", "remove_dir"},
			keep:       map[string]struct{}{"keep.txt": {}},
			wantRemain: []string{"keep.txt"},
			wantErr:    nil,
		},
		{
			name:       "empty_keep_map",
			setupFiles: []string{"file1.txt", "dir1"},
			keep:       map[string]struct{}{},
			wantRemain: []string{},
			wantErr:    nil,
		},
		{
			name:       "nil_keep_map",
			setupFiles: []string{"file1.txt", "dir1"},
			keep:       nil,
			wantRemain: []string{},
			wantErr:    nil,
		},
		{
			name:       "keep_all",
			setupFiles: []string{"fileA.txt", "fileB.txt"},
			keep:       map[string]struct{}{"fileA.txt": {}, "fileB.txt": {}},
			wantRemain: []string{"fileA.txt", "fileB.txt"},
			wantErr:    nil,
		},
		{
			name:       "empty_directory",
			setupFiles: []string{},
			keep:       map[string]struct{}{"nonexistent.txt": {}},
			wantRemain: []string{},
			wantErr:    nil,
		},
		{
			name:       "invalid_directory",
			setupFiles: []string{},
			keep:       map[string]struct{}{},
			useBadDir:  true,
			wantRemain: []string{},
			wantErr:    os.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			for _, f := range tt.setupFiles {
				path := filepath.Join(tempDir, f)
				if filepath.Ext(f) == "" {
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatalf("failed to setup dir: %v", err)
					}
				} else {
					if err := os.WriteFile(path, []byte("test content"), 0o644); err != nil {
						t.Fatalf("failed to setup file: %v", err)
					}
				}
			}

			targetDir := tempDir
			if tt.useBadDir {
				targetDir = filepath.Join(tempDir, "does_not_exist")
			}

			err := removeAllExcept(targetDir, tt.keep)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("removeAllExcept() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("removeAllExcept() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("removeAllExcept() unexpected error: %v", err)
			}

			entries, err := os.ReadDir(tempDir)
			if err != nil {
				t.Fatalf("failed to read dir after removeAllExcept: %v", err)
			}

			gotRemain := []string{}
			for _, e := range entries {
				gotRemain = append(gotRemain, e.Name())
			}

			sort.Strings(gotRemain)
			sort.Strings(tt.wantRemain)

			if !reflect.DeepEqual(gotRemain, tt.wantRemain) {
				t.Fatalf("removeAllExcept() left files = %v; want %v", gotRemain, tt.wantRemain)
			}
		})
	}
}
