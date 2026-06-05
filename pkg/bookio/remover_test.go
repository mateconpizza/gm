package bookio

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var _ FileManager = (*MockFileManager)(nil)

type MockFileManager struct {
	ExistsErrFn       func(path string) error
	RemoveFn          func(path string) error
	RemoveEmptyDirsFn func(root string) error
}

func (m *MockFileManager) ExistsErr(path string) error {
	if m.ExistsErrFn != nil {
		return m.ExistsErrFn(path)
	}
	return nil
}

func (m *MockFileManager) Rm(path string) error {
	if m.RemoveFn != nil {
		return m.RemoveFn(path)
	}
	return nil
}

func (m *MockFileManager) RmEmptyDirs(root string) error {
	if m.RemoveEmptyDirsFn != nil {
		return m.RemoveEmptyDirsFn(root)
	}
	return nil
}

func TestNewFileRemover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		root       string
		fm         FileManager
		genFn      GenFullpathFn
		wantErr    bool
		wantErrMsg string
	}{
		{
			"valid_creation",
			"/repo",
			&MockFileManager{},
			func(root string, b *bookmark.Bookmark) (string, error) { return "/repo/file", nil },
			false,
			"",
		},
		{
			"empty_root",
			"",
			&MockFileManager{},
			func(root string, b *bookmark.Bookmark) (string, error) { return "", nil },
			true,
			"root path cannot be empty",
		},
		{
			"nil_file_manager",
			"/repo",
			nil,
			func(root string, b *bookmark.Bookmark) (string, error) { return "", nil },
			true,
			"file manager cannot be nil",
		},
		{
			"nil_genFullpath",
			"/repo",
			&MockFileManager{},
			nil,
			true,
			"genFullpath function cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fr, err := NewFileRemover(tt.root, tt.fm, tt.genFn)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewFileRemover() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error message = %q; want contains %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewFileRemover() unexpected error: %v", err)
			}
			if fr == nil {
				t.Fatal("NewFileRemover() returned nil")
			}
		})
	}
}

func TestFileRemoverRm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		root        string
		bookmarks   []*bookmark.Bookmark
		genFullpath GenFullpathFn
		mockFM      *MockFileManager
		wantErr     bool
		wantErrMsg  string
	}{
		{
			"remove_single_file",
			"/repo",
			[]*bookmark.Bookmark{{ID: 1, Title: "test"}},
			func(root string, b *bookmark.Bookmark) (string, error) {
				return "/repo/test.json", nil
			},
			&MockFileManager{
				ExistsErrFn:       func(path string) error { return nil },
				RemoveFn:          func(path string) error { return nil },
				RemoveEmptyDirsFn: func(root string) error { return nil },
			},
			false,
			"",
		},
		{
			"file_not_found",
			"/repo",
			[]*bookmark.Bookmark{{ID: 1}},
			func(root string, b *bookmark.Bookmark) (string, error) {
				return "/repo/notfound.json", nil
			},
			&MockFileManager{
				ExistsErrFn: func(path string) error { return nil },
				RemoveFn: func(path string) error {
					return os.ErrNotExist
				},
				RemoveEmptyDirsFn: func(root string) error { return nil },
			},
			false,
			"",
		},
		{
			"remove_fails",
			"/repo",
			[]*bookmark.Bookmark{{ID: 1}},
			func(root string, b *bookmark.Bookmark) (string, error) {
				return "/repo/test.json", nil
			},
			&MockFileManager{
				ExistsErrFn: func(path string) error { return nil },
				RemoveFn: func(path string) error {
					return errors.New("permission denied")
				},
				RemoveEmptyDirsFn: func(root string) error { return nil },
			},
			true,
			"removing",
		},
		{
			"cleanup_fails",
			"/repo",
			[]*bookmark.Bookmark{{ID: 1}},
			func(root string, b *bookmark.Bookmark) (string, error) {
				return "/repo/test.json", nil
			},
			&MockFileManager{
				ExistsErrFn: func(path string) error { return nil },
				RemoveFn:    func(path string) error { return nil },
				RemoveEmptyDirsFn: func(root string) error {
					return errors.New("cleanup error")
				},
			},
			true,
			"removing empty dirs",
		},
		{
			"empty_bookmarks",
			"/repo",
			[]*bookmark.Bookmark{},
			func(root string, b *bookmark.Bookmark) (string, error) {
				return "/repo/test.json", nil
			},
			&MockFileManager{},
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fr, _ := NewFileRemover(tt.root, tt.mockFM, tt.genFullpath)
			err := fr.Rm(t.Context(), tt.bookmarks)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Rm() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q; want contains %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Rm() unexpected error: %v", err)
			}
		})
	}
}
