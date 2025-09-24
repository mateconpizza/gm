package locker

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/files"
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

		if !errors.Is(err, files.ErrFileNotFound) {
			t.Errorf("Expected files.ErrFileNotFound, got %v", err)
		}
	})
}

func TestBackupFile(t *testing.T) {
	t.Parallel()
	tf := testTempFile(t)

	defer func() {
		if err := os.Remove(tf.Name()); err != nil {
			slog.Error("err removing tempfile", "error", err.Error())
		}
	}()

	b := []byte(
		"Lorem ipsum dolor sit amet, qui minim labore adipisicing minim sint cillum sint consectetur cupidatat.",
	)
	err := os.WriteFile(tf.Name(), b, files.FilePerm)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	//nolint:paralleltest //fails
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

		if !errors.Is(err, files.ErrFileNotFound) {
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
	ciphertext, err := encrypt(b, pp)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	plaintext, err := decrypt(ciphertext, pp)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, b) {
		t.Errorf("Decrypted text doesn't match original. Expected %q, got %q", string(b), string(plaintext))
	}
}

//nolint:funlen //test
func TestWriteAndReplaceFile(t *testing.T) {
	t.Parallel()
	// Helper function to create and write to a file for testing
	createTestFile := func(t *testing.T, content string) string {
		t.Helper()
		tf := testTempFile(t)

		err := os.WriteFile(tf.Name(), []byte(content), files.FilePerm)
		if err != nil {
			t.Fatal(err)
		}

		return tf.Name()
	}

	// Helper function to read a file's content
	readFileContent := func(t *testing.T, path string) string {
		t.Helper()

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		return string(content)
	}

	// Helper function to check if a file exists
	fileExists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	t.Run("successful write and replace", func(t *testing.T) {
		t.Parallel()
		// Create original file
		originalPath := createTestFile(t, "original content")
		// Create backup file
		backupPath := createTestFile(t, "backup content")
		// Define target path
		targetPath := filepath.Join(t.TempDir(), "target.txt")
		// Test data to write
		data := []byte("new content")
		// Execute the function
		err := writeAndReplaceFile(targetPath, data, originalPath, backupPath)
		// Assertions
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !fileExists(targetPath) {
			t.Error("Target file should exist")
		}
		if fileExists(originalPath) {
			t.Error("Original file should be removed")
		}
		content := readFileContent(t, targetPath)
		if content != "new content" {
			t.Errorf("Target file should contain new content, got %q", content)
		}
	})

	t.Run("fails when backup restore fails", func(t *testing.T) {
		t.Parallel()
		// Setup test files
		original := testTempFile(t)
		backup := "/invalid/backup/path" // Will cause restore to fail
		target := "/non/existent/path"   // Will cause write to fail

		// Execute
		err := writeAndReplaceFile(target, []byte("new content"), original.Name(), backup)

		// Verify
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "failed to write file and restore backup") {
			t.Errorf("Error should contain 'failed to write file and restore backup', got %v", err)
		}
		if err != nil && !strings.Contains(err.Error(), "original error") {
			t.Errorf("Error should contain 'original error', got %v", err)
		}
	})

	t.Run("failed write restores from backup", func(t *testing.T) {
		t.Parallel()
		// Setup test files
		original := testTempFile(t)
		backup := testTempFile(t)
		target := "/non/existent/path" // Will cause write to fail

		// Write initial content to original and backup
		originalContent := []byte("original content")
		err := os.WriteFile(original.Name(), originalContent, files.FilePerm)
		if err != nil {
			t.Fatalf("Failed to write original file: %v", err)
		}
		err = os.WriteFile(backup.Name(), originalContent, files.FilePerm)
		if err != nil {
			t.Fatalf("Failed to write backup file: %v", err)
		}

		// Execute
		err = writeAndReplaceFile(target, []byte("new content"), original.Name(), backup.Name())

		// Verify
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "failed to write file") {
			t.Errorf("Error should contain 'failed to write file', got %v", err)
		}

		// Check original was restored
		currentContent, err := os.ReadFile(original.Name())
		if err != nil {
			t.Fatalf("Failed to read original file: %v", err)
		}

		if !bytes.Equal(currentContent, originalContent) {
			t.Errorf(
				"Original content not restored. Expected %q, got %q",
				string(originalContent),
				string(currentContent),
			)
		}
	})
}
