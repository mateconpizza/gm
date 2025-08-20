package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, want, got)
	})

	t.Run("no suffixes", func(t *testing.T) {
		t.Parallel()

		want := "somefile"
		got := StripSuffixes(want)
		assert.Equal(t, want, got)
	})
}
