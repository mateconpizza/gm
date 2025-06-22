package locker

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mateconpizza/gm/internal/sys/files"
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
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPassphraseEmpty)
	})

	t.Run("valid passphrase", func(t *testing.T) {
		t.Parallel()

		tf := createTestFile(t, "")
		pass := "123"
		err := validateInput(tf, pass)
		assert.NoError(t, err)
	})

	t.Run("invalid filepath", func(t *testing.T) {
		t.Parallel()

		pass := "123456"
		err := validateInput("/tmp/invalid/path", pass)
		assert.Error(t, err)
		assert.ErrorIs(t, err, files.ErrFileNotFound)
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
	assert.NoError(t, err)
	//nolint:paralleltest //fails
	t.Run("valid filepath", func(t *testing.T) {
		_, err := backupFile(tf.Name())
		assert.NoError(t, err)
	})

	t.Run("invalid filepath", func(t *testing.T) {
		t.Parallel()

		_, err := backupFile("/tmp/invalid/path")
		assert.Error(t, err)
		assert.ErrorIs(t, err, files.ErrFileNotFound)
	})
}

func TestLockAndUnlocked(t *testing.T) {
	t.Parallel()

	pp := "123456"
	b := []byte(
		"Lorem ipsum dolor sit amet, qui minim labore adipisicing minim sint cillum sint consectetur cupidatat.",
	)
	ciphertext, err := encrypt(b, pp)
	assert.NoError(t, err)
	plaintext, err := decrypt(ciphertext, pp)
	assert.NoError(t, err)
	assert.Equal(t, b, plaintext)
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
		assert.NoError(t, err)
		assert.True(t, fileExists(targetPath), "Target file should exist")
		assert.False(t, fileExists(originalPath), "Original file should be removed")
		assert.Equal(
			t,
			"new content",
			readFileContent(t, targetPath),
			"Target file should contain new content",
		)
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write file and restore backup")
		assert.Contains(t, err.Error(), "original error")
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
		assert.NoError(t, err)
		err = os.WriteFile(backup.Name(), originalContent, files.FilePerm)
		assert.NoError(t, err)

		// Execute
		err = writeAndReplaceFile(target, []byte("new content"), original.Name(), backup.Name())

		// Verify
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write file")

		// Check original was restored
		currentContent, err := os.ReadFile(original.Name())
		assert.NoError(t, err)
		assert.Equal(t, originalContent, currentContent)
	})
}
