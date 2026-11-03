// Package locker provides AES-GCM encryption and decryption utilities for
// files, including backup and restore logic.
package locker

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	files "github.com/mateconpizza/gofiles"
	"golang.org/x/crypto/argon2"
)

var (
	ErrPassphraseEmpty    = errors.New("passphrase cannot be empty")
	ErrPassphraseMismatch = errors.New("passphrase mismatch")
	ErrPassphraseInvalid  = errors.New("invalid passphrase")
	ErrFileLocked         = errors.New("file is locked")
	ErrFileUnlocked       = errors.New("file is unlocked")
	ErrFileExtMismatch    = errors.New("file must have .enc extension")
	ErrCipherTextShort    = errors.New("ciphertext too short")
)

// gitAttContent defines the Git attributes rule for encrypted files.
const gitAttContent = "*.enc binary"

type Ext string

func (e Ext) Join(filename string) string {
	return files.EnsureExt(filename, string(e))
}

func (e Ext) String() string {
	return string(e)
}

const Extension Ext = ".enc"

const (
	saltSize     = 16
	keySize      = 32 // AES-256
	argonTime    = 1
	argonMem     = 64 * 1024 // 64 MB
	argonThreads = 4
)

// deriveKey derives a 32-byte key from passphrase and salt using Argon2id.
func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMem, argonThreads, keySize)
}

// Lock encrypts the given file using AES-GCM encryption and adds .enc
// extension.
func Lock(ctx context.Context, path, passphrase string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

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
	ciphertext, err := encrypt(ctx, plaintext, passphrase)
	if err != nil {
		return err
	}

	// Write encrypted data to disk
	lockedPath := Extension.Join(path)

	err = writeAndReplaceFile(lockedPath, ciphertext, path, backupPath)
	if err != nil {
		return err
	}

	slog.Debug("file locked", "path", lockedPath)

	// Cleanup successful operation
	_ = os.Remove(backupPath)

	return nil
}

// Unlock decrypts the given .enc file using AES-GCM decryption and removes the
// .enc extension.
func Unlock(ctx context.Context, path, passphrase string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	slog.Debug("unlocking file", "path", path)

	if err := validateInput(path, passphrase); err != nil {
		return err
	}

	if !strings.HasSuffix(path, Extension.String()) {
		return fmt.Errorf("%w: got %q, path: %q", ErrFileExtMismatch, filepath.Ext(path), path)
	}

	// Read the encrypted content
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read locked file: %w", err)
	}

	// Perform decryption
	plaintext, err := Decrypt(ctx, ciphertext, passphrase)
	if err != nil {
		return err
	}

	// Create a backup before modifying files
	backupPath, err := backupFile(path)
	if err != nil {
		return fmt.Errorf("backup creation failed: %w", err)
	}

	// Write decrypted data to disk
	decryptedPath := strings.TrimSuffix(path, Extension.String())

	err = writeAndReplaceFile(decryptedPath, plaintext, path, backupPath)
	if err != nil {
		return err
	}

	slog.Debug("file unlocked", "path", decryptedPath)

	// Cleanup successful operation
	_ = os.Remove(backupPath)

	return nil
}

// IsLocked checks if the given file has .enc extension.
func IsLocked(s string) error {
	slog.Debug("checking if file is locked")

	s = Extension.Join(s)
	if files.Exists(s) {
		return fmt.Errorf("%w: %q", ErrFileLocked, filepath.Base(s))
	}

	slog.Debug("file not locked")

	return nil
}

func Init(path, gitAttrFile string) error {
	if err := os.MkdirAll(path, files.DirPerm); err != nil {
		return fmt.Errorf("%w", err)
	}

	gf, err := files.NewWithData(filepath.Join(path, gitAttrFile), []byte(gitAttContent), true)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return gf.Close()
}

// encrypt encrypts data using AES-GCM with the given passphrase.
func encrypt(ctx context.Context, plaintext []byte, passphrase string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("salt generation failed: %w", err)
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM mode creation failed: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation failed: %w", err)
	}

	// layout: salt || nonce || ciphertext
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

// Decrypt decrypts data using AES-GCM with the given passphrase.
func Decrypt(ctx context.Context, data []byte, passphrase string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(data) < saltSize {
		return nil, ErrCipherTextShort
	}
	salt, rest := data[:saltSize], data[saltSize:]

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM mode creation failed: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize {
		return nil, ErrCipherTextShort
	}
	nonce, data := rest[:nonceSize], rest[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		if !strings.Contains(err.Error(), "cipher: message authentication failed") {
			return nil, fmt.Errorf("decryption failed: %w", err)
		}
		return nil, ErrPassphraseInvalid
	}

	return plaintext, nil
}

// IsValidPassphrase checks if the passphrase can successfully decrypt the
// file.
func IsValidPassphrase(ctx context.Context, path, passphrase string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = Decrypt(ctx, data, passphrase)
	if err != nil {
		return err
	}
	return nil
}

// validateInput validates input parameters for Lock function.
func validateInput(path, passphrase string) error {
	if passphrase == "" {
		return ErrPassphraseEmpty
	}

	if !files.Exists(path) {
		return fmt.Errorf("%w: %s", os.ErrNotExist, path)
	}

	return nil
}

// backupFile creates a backup of the given file.
func backupFile(filePath string) (string, error) {
	if !files.Exists(filePath) {
		return "", fmt.Errorf("%w: %s", os.ErrNotExist, filePath)
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
	//nolint:gosec //false positive
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
	//nolint:gosec //false positive
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
