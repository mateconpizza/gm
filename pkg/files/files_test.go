//nolint:thelper,gocyclo //unnecessary
package files

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExists(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	existingFilePath := filepath.Join(tempDir, "existing-file.txt")
	if _, err := os.Create(existingFilePath); err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "FileExists",
			path:     existingFilePath,
			expected: true,
		},
		{
			name:     "FileDoesNotExist",
			path:     filepath.Join(tempDir, "non-existent-file.txt"),
			expected: false,
		},
		{
			name:     "DirectoryExists",
			path:     tempDir,
			expected: true,
		},
		{
			name:     "EmptyString",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Exists(tt.path)
			if got != tt.expected {
				t.Errorf("Exists(%q) = %v; expected %v", tt.path, got, tt.expected)
			}
		})
	}
}

// TestExistsErr uses a table-driven approach to test various scenarios.
func TestExistsErr(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	existingFilePath := filepath.Join(tempDir, "existing-file.txt")
	if _, err := os.Create(existingFilePath); err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected error
	}{
		{
			name:     "FileExists",
			path:     existingFilePath,
			expected: nil,
		},
		{
			name:     "FileDoesNotExist",
			path:     filepath.Join(tempDir, "non-existent-file.txt"),
			expected: ErrFileNotFound,
		},
		{
			name:     "DirectoryExists",
			path:     tempDir,
			expected: nil,
		},
		{
			name:     "EmptyString",
			path:     "",
			expected: ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExistsErr(tt.path)

			if tt.expected != nil {
				if got == nil {
					t.Errorf("ExistsErr(%q) = nil; expected %v", tt.path, tt.expected)
				} else if got.Error() != tt.expected.Error() {
					t.Errorf("ExistsErr(%q) = %v; expected %v", tt.path, got, tt.expected)
				}
			} else if got != nil {
				t.Errorf("ExistsErr(%q) = %v; expected nil", tt.path, got)
			}
		})
	}
}

func TestIsFile(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "existing-file.txt")
	if _, err := os.Create(existingFile); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	nonExistentPath := filepath.Join(tempDir, "non-existent-path")

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Is a file",
			path:     existingFile,
			expected: true,
		},
		{
			name:     "Is a directory",
			path:     tempDir,
			expected: false,
		},
		{
			name:     "Does not exist",
			path:     nonExistentPath,
			expected: false,
		},
		{
			name:     "Empty string path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsFile(tt.path)
			if got != tt.expected {
				t.Errorf("IsFile(%q) got %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestStripSuffixes(t *testing.T) {
	t.Run("remove all suffixes", func(t *testing.T) {
		t.Parallel()

		want := "somefile"
		p := want + ".db.enc"
		got := StripSuffixes(p)
		if got != want {
			t.Errorf("StripSuffixes(%q) = %q, want %q", p, got, want)
		}
	})

	t.Run("no suffixes", func(t *testing.T) {
		t.Parallel()

		want := "somefile"
		got := StripSuffixes(want)
		if got != want {
			t.Errorf("StripSuffixes(%q) = %q, want %q", want, got, want)
		}
	})
}

//nolint:gocognit,funlen //testing
func TestTouch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string) string // returns file path
		existsOK  bool
		wantErr   bool
		checkErr  func(t *testing.T, err error)
		checkFile func(t *testing.T, path string, f *os.File)
	}{
		{
			name: "creates new file successfully",
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "newfile.txt")
			},
			existsOK: false,
			wantErr:  false,
			checkFile: func(t *testing.T, path string, f *os.File) {
				if f == nil {
					t.Fatal("expected file handle, got nil")
				}
				if !Exists(path) {
					t.Errorf("file should exist at %q", path)
				}
			},
		},
		{
			name: "creates file in nested non-existent directory",
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "nested", "deep", "file.txt")
			},
			existsOK: false,
			wantErr:  false,
			checkFile: func(t *testing.T, path string, f *os.File) {
				if f == nil {
					t.Fatal("expected file handle, got nil")
				}
				if !Exists(path) {
					t.Errorf("file should exist at %q", path)
				}
				if !Exists(filepath.Dir(path)) {
					t.Errorf("parent directory should exist")
				}
			},
		},
		{
			name: "fails when file exists and existsOK is false",
			setup: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "existing.txt")
				if _, err := os.Create(path); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return path
			},
			existsOK: false,
			wantErr:  true,
			checkErr: func(t *testing.T, err error) {
				if !errors.Is(err, ErrFileExists) {
					t.Errorf("expected ErrFileExists, got %v", err)
				}
				if !strings.Contains(err.Error(), "existing.txt") {
					t.Errorf("error should contain filename, got %q", err.Error())
				}
			},
		},
		{
			name: "succeeds when file exists and existsOK is true",
			setup: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "existing.txt")
				if _, err := os.Create(path); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				// Write some content to verify it gets truncated
				if err := os.WriteFile(path, []byte("old content"), FilePerm); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return path
			},
			existsOK: true,
			wantErr:  false,
			checkFile: func(t *testing.T, path string, f *os.File) {
				if f == nil {
					t.Fatal("expected file handle, got nil")
				}
				// Verify file was truncated (os.Create truncates)
				info, err := f.Stat()
				if err != nil {
					t.Fatalf("failed to stat file: %v", err)
				}
				if info.Size() != 0 {
					t.Errorf("expected file to be truncated, got size %d", info.Size())
				}
			},
		},
		{
			name: "fails with invalid path characters",
			setup: func(t *testing.T, dir string) string {
				//nolint:gocritic //testing
				return filepath.Join(dir, "invalid\x00name.txt")
			},
			existsOK: false,
			wantErr:  true,
			checkErr: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("expected error with invalid path")
				}
			},
		},
		{
			name: "fails when parent is a file not a directory",
			setup: func(t *testing.T, dir string) string {
				parentFile := filepath.Join(dir, "parent")
				if _, err := os.Create(parentFile); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return filepath.Join(parentFile, "child.txt")
			},
			existsOK: false,
			wantErr:  true,
			checkErr: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("expected error when parent is a file")
				}
			},
		},
		{
			name: "creates file in existing directory",
			setup: func(t *testing.T, dir string) string {
				subdir := filepath.Join(dir, "existing-dir")
				if err := os.MkdirAll(subdir, DirPerm); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return filepath.Join(subdir, "newfile.txt")
			},
			existsOK: false,
			wantErr:  false,
			checkFile: func(t *testing.T, path string, f *os.File) {
				if f == nil {
					t.Fatal("expected file handle, got nil")
				}
				if !Exists(path) {
					t.Errorf("file should exist at %q", path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := tt.setup(t, dir)

			f, err := Touch(path, tt.existsOK)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Always close the file if returned
			if f != nil {
				defer func() {
					if err := f.Close(); err != nil {
						slog.Error("closing source file", "file", f.Name(), "error", err)
					}
				}()
			}

			if tt.checkFile != nil {
				tt.checkFile(t, path, f)
			}
		})
	}
}
