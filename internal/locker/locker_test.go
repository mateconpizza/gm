package locker

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	files "github.com/mateconpizza/gofiles"
)

func testTempFile(t *testing.T) *os.File {
	t.Helper()

	tf, err := os.CreateTemp(t.TempDir(), "labex-*.txt")
	if err != nil {
		t.Fatal(err)
	}

	return tf
}

func TestValidateInput(t *testing.T) {
	t.Parallel()

	createTestFile := func(t *testing.T, content string) string {
		t.Helper()
		tf := testTempFile(t)

		err := os.WriteFile(tf.Name(), []byte(content), files.FilePerm)
		if err != nil {
			t.Fatal(err)
		}

		return tf.Name()
	}

	t.Run("empty|invalid passphrase", func(t *testing.T) {
		t.Parallel()

		tf := createTestFile(t, "")
		pass := ""
		err := validateInput(tf, pass)
		if err == nil {
			t.Error("Expected error for empty passphrase, got nil")
		}

		if !errors.Is(err, ErrPassphraseEmpty) {
			t.Errorf("Expected ErrPassphraseEmpty, got %v", err)
		}
	})

	t.Run("valid passphrase", func(t *testing.T) {
		t.Parallel()

		tf := createTestFile(t, "")
		pass := "123"
		err := validateInput(tf, pass)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("invalid filepath", func(t *testing.T) {
		t.Parallel()

		pass := "123456"
		err := validateInput("/tmp/invalid/path", pass)
		if err == nil {
			t.Error("Expected error for invalid filepath, got nil")
		}

		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Expected files.ErrFileNotFound, got %v", err)
		}
	})
}

func TestBackupFile(t *testing.T) {
	t.Parallel()
	tf := testTempFile(t)
	t.Cleanup(func() {
		os.Remove(tf.Name())
	})

	b := []byte(
		"Lorem ipsum dolor sit amet, qui minim labore adipisicing minim sint cillum sint consectetur cupidatat.",
	)
	err := os.WriteFile(tf.Name(), b, files.FilePerm)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	t.Run("valid filepath", func(t *testing.T) {
		_, err := backupFile(tf.Name())
		if err != nil {
			t.Errorf("Expected no error for valid filepath, got %v", err)
		}
	})

	t.Run("invalid filepath", func(t *testing.T) {
		t.Parallel()

		_, err := backupFile("/tmp/invalid/path")
		if err == nil {
			t.Error("Expected error for invalid filepath, got nil")
		}

		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Expected files.ErrFileNotFound, got %v", err)
		}
	})
}

func TestLockAndUnlocked(t *testing.T) {
	t.Parallel()

	pp := "123456"
	b := []byte(
		"Lorem ipsum dolor sit amet, qui minim labore adipisicing minim sint cillum sint consectetur cupidatat.",
	)
	ciphertext, err := encrypt(t.Context(), b, pp)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	plaintext, err := Decrypt(t.Context(), ciphertext, pp)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, b) {
		t.Errorf("Decrypted text doesn't match original. Expected %q, got %q", string(b), string(plaintext))
	}
}

func TestWriteAndReplaceFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   string // relative to the temp dir; used as a raw suffix, so "./x" stays uncleaned
		original string
		backup   string
		data     []byte
		seed     map[string]string // files created before the call
		want     map[string]string // files that must exist afterwards, with exact content
		wantGone []string          // files that must not exist afterwards
		wantErr  error
		wantMsg  string // substring of the error message
	}{
		{
			name:     "replaces_original_with_target",
			target:   "new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			seed:     map[string]string{"orig.txt": "old", "bak.txt": "bak"},
			want:     map[string]string{"new.txt": "new", "bak.txt": "bak"},
			wantGone: []string{"orig.txt"},
		},
		{
			name:     "same_path_keeps_file",
			target:   "a.txt",
			original: "a.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			seed:     map[string]string{"a.txt": "old", "bak.txt": "bak"},
			want:     map[string]string{"a.txt": "new", "bak.txt": "bak"},
		},
		{
			name:     "same_path_uncleaned_keeps_file",
			target:   "./a.txt",
			original: "a.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			seed:     map[string]string{"a.txt": "old", "bak.txt": "bak"},
			want:     map[string]string{"a.txt": "new", "bak.txt": "bak"},
		},
		{
			name:     "nil_data_creates_empty_file",
			target:   "new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     nil,
			seed:     map[string]string{"orig.txt": "old", "bak.txt": "bak"},
			want:     map[string]string{"new.txt": "", "bak.txt": "bak"},
			wantGone: []string{"orig.txt"},
		},
		{
			name:     "write_error_restores_backup",
			target:   "missing/new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			seed:     map[string]string{"bak.txt": "bak"},
			want:     map[string]string{"orig.txt": "bak"},
			wantGone: []string{"bak.txt"},
			wantErr:  fs.ErrNotExist,
			wantMsg:  "failed to write file",
		},
		{
			name:     "write_error_restore_also_fails",
			target:   "missing/new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			wantGone: []string{"orig.txt", "bak.txt"},
			wantErr:  fs.ErrNotExist,
			wantMsg:  "failed to write file and restore backup",
		},
		{
			name:     "remove_error_restores_backup",
			target:   "new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			seed:     map[string]string{"bak.txt": "bak"},
			want:     map[string]string{"new.txt": "new", "orig.txt": "bak"},
			wantGone: []string{"bak.txt"},
			wantErr:  fs.ErrNotExist,
			wantMsg:  "failed to remove original file",
		},
		{
			name:     "remove_error_restore_also_fails",
			target:   "new.txt",
			original: "orig.txt",
			backup:   "bak.txt",
			data:     []byte("new"),
			want:     map[string]string{"new.txt": "new"},
			wantGone: []string{"orig.txt", "bak.txt"},
			wantErr:  fs.ErrNotExist,
			wantMsg:  "failed to remove original file and restore backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for name, content := range tt.seed {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatalf("seed %s: %v", name, err)
				}
			}

			err := writeAndReplaceFile(
				dir+"/"+tt.target,
				tt.data,
				filepath.Join(dir, tt.original),
				filepath.Join(dir, tt.backup),
			)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v; want errors.Is %v", err, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("error = %q; want it to contain %q", err.Error(), tt.wantMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for name, wantContent := range tt.want {
				got, readErr := os.ReadFile(filepath.Join(dir, name))
				if readErr != nil {
					t.Fatalf("expected %s to exist: %v", name, readErr)
				}
				if string(got) != wantContent {
					t.Fatalf("%s = %q; want %q", name, got, wantContent)
				}
			}

			for _, name := range tt.wantGone {
				if _, statErr := os.Stat(filepath.Join(dir, name)); !errors.Is(statErr, fs.ErrNotExist) {
					t.Fatalf("expected %s to be gone, stat error = %v", name, statErr)
				}
			}
		})
	}
}
