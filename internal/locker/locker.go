package locker

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/sys/files"
)

var (
	ErrPassphraseEmpty    = errors.New("password cannot be empty")
	ErrPassphraseMismatch = errors.New("password mismatch")
	ErrItemLocked         = errors.New("item is locked")
	ErrItemUnlocked       = errors.New("item is unlocked")
	ErrFileExtMismatch    = errors.New("file must have .enc extension")
	ErrCipherTextShort    = errors.New("ciphertext too short")
)

// Lock encrypts the given file using AES-GCM encryption and adds .enc
// extension.
func Lock(path, passphrase string) error {
	slog.Debug("locking file", "path", path)

	if err := validateInput(path, passphrase); err != nil {
		return err
	}
	// Create a backup
	backupPath, err := backupFile(path)
	if err != nil {
		return fmt.Errorf("backup creation failed: %w", err)
	}
	// Read the file content
	plaintext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	// Perform encryption
	ciphertext, err := encrypt(plaintext, passphrase)
	if err != nil {
		return err
	}
	// Write encrypted data to disk
	lockedPath := path + ".enc"

	err = writeAndReplaceFile(lockedPath, ciphertext, path, backupPath)
	if err != nil {
		return err
	}

	slog.Debug("file locked", "path", lockedPath)
	// Cleanup successful operation
	_ = os.Remove(backupPath)

	return nil
}

// Unlock decrypts the given .enc file using AES-GCM decryption and removes the .enc extension.
func Unlock(path, passphrase string) error {
	slog.Debug("unlocking file", "path", path)

	if err := validateInput(path, passphrase); err != nil {
		return err
	}

	if !strings.HasSuffix(path, ".enc") {
		return fmt.Errorf("%w: got %q", ErrFileExtMismatch, filepath.Ext(path))
	}
	// Read the encrypted content
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read locked file: %w", err)
	}
	// Perform decryption
	plaintext, err := decrypt(ciphertext, passphrase)
	if err != nil {
		return err
	}
	// Create a backup before modifying files
	backupPath, err := backupFile(path)
	if err != nil {
		return fmt.Errorf("backup creation failed: %w", err)
	}
	// Write decrypted data to disk
	decryptedPath := strings.TrimSuffix(path, ".enc")

	err = writeAndReplaceFile(decryptedPath, plaintext, path, backupPath)
	if err != nil {
		return err
	}

	slog.Debug("file unlocked", "path", decryptedPath)
	// Cleanup successful operation
	_ = os.Remove(backupPath)

	return nil
}

// encrypt encrypts data using AES-GCM with the given passphrase.
func encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	// Generate a key from the passphrase
	key := generateKey(passphrase)
	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %w", err)
	}
	// Create a new GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM mode creation failed: %w", err)
	}
	// Create a nonce (number used once)
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation failed: %w", err)
	}
	// Encrypt the data and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM with the given passphrase.
func decrypt(ciphertext []byte, passphrase string) ([]byte, error) {
	// Generate the key from the passphrase
	key := generateKey(passphrase)
	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %w", err)
	}
	// Create a new GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM mode creation failed: %w", err)
	}
	// Verify the ciphertext is long enough
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCipherTextShort
	}
	// Extract the nonce
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// IsLocked checks if the given file has .enc extension.
func IsLocked(s string) error {
	slog.Debug("checking if file is locked")

	if !strings.HasSuffix(s, ".enc") {
		s += ".enc"
	}

	if files.Exists(s) {
		return fmt.Errorf("%w: %q", ErrItemLocked, filepath.Base(s))
	}

	slog.Debug("file not locked")

	return nil
}

// generateKey creates a 32-byte key from the passphrase using SHA-256.
func generateKey(passphrase string) []byte {
	slog.Debug("generating key from passphrase")

	hash := sha256.Sum256([]byte(passphrase))

	return hash[:]
}

// validateInput validates input parameters for Lock function.
func validateInput(path, passphrase string) error {
	if passphrase == "" {
		return ErrPassphraseEmpty
	}

	if !files.Exists(path) {
		return fmt.Errorf("%w: %s", files.ErrFileNotFound, path)
	}

	return nil
}

// backupFile creates a backup of the given file.
func backupFile(filePath string) (string, error) {
	if !files.Exists(filePath) {
		return "", fmt.Errorf("%w: %s", files.ErrFileNotFound, filePath)
	}
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup_%s", filePath, timestamp)

	// Read the original file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Write backup file
	err = os.WriteFile(backupPath, data, files.FilePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}

	slog.Debug("created backup file", "path", backupPath)

	return backupPath, nil
}

// writeAndReplaceFile writes data to targetPath, removes originalPath on
// success, and handles error recovery using backupPath.
func writeAndReplaceFile(targetPath string, data []byte, originalPath, backupPath string) error {
	slog.Debug("writing file", "path", targetPath)
	// Write data to the target file
	err := os.WriteFile(targetPath, data, files.FilePerm)
	if err != nil {
		// If writing fails, attempt to restore from backup
		slog.Debug("restore from backup", "path", backupPath)

		restoreErr := os.Rename(backupPath, originalPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"failed to write file and restore backup: %w (original error: %w)",
				restoreErr,
				err,
			)
		}

		return fmt.Errorf("failed to write file: %w", err)
	}

	slog.Debug("replacing file", "original", originalPath, "target", targetPath)
	// Remove the original file
	err = os.Remove(originalPath)
	if err != nil {
		// If removal fails, attempt to restore from backup
		restoreErr := os.Rename(backupPath, originalPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"failed to remove original file and restore backup: %w (original error: %w)",
				restoreErr,
				err,
			)
		}

		return fmt.Errorf("failed to remove original file: %w", err)
	}

	slog.Debug("wrote and replaced file", "path", targetPath)

	return nil
}
