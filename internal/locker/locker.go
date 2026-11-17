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

type Ext string

func (e Ext) Join(filename string) string {
	if strings.HasSuffix(filename, e.String()) {
		return filename
	}
	return filename + e.String()
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
func Lock(ctx context.Context, path, passphrase string, logger *slog.Logger) error {
	log := logger.With(
		"cmp", "locker",
		"fn", "lock",
		"file", filepath.Base(path),
	)

	log.Debug("locking file")

	if err := ctx.Err(); err != nil {
		log.Debug("locking canceled", "error", err)
		return err
	}

	if err := validateInput(path, passphrase); err != nil {
		log.Warn("invalid lock input", "error", err)
		return err
	}

	// Create a backup.
	backupPath, err := backupFile(log, path)
	if err != nil {
		log.Error("failed to create backup", "error", err)
		return fmt.Errorf("backup creation failed: %w", err)
	}

	log.Debug("backup created", "backup", filepath.Base(backupPath))

	// Read the file content.
	plaintext, err := os.ReadFile(path)
	if err != nil {
		log.Error("failed to read input file", "error", err)
		return fmt.Errorf("failed to read input file: %w", err)
	}

	log.Debug("input file read", "size", len(plaintext))

	// Perform encryption.
	ciphertext, err := encrypt(ctx, plaintext, passphrase)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Debug("encryption canceled", "error", ctxErr)
			return ctxErr
		}

		log.Error("failed to encrypt file", "error", err)
		return err
	}

	log.Debug("file encrypted", "size", len(ciphertext))

	// Write encrypted data to disk.
	lockedPath := Extension.Join(path)

	if err := writeAndReplaceFile(log, lockedPath, ciphertext, path, backupPath); err != nil {
		log.Error("failed to replace input file", "error", err)
		return err
	}

	log.Debug("file locked successfully", "output", filepath.Base(lockedPath))

	// Cleanup successful operation.
	if err := os.Remove(backupPath); err != nil {
		log.Warn("failed to remove temporary backup",
			"backup", filepath.Base(backupPath),
			"error", err,
		)
	}

	return nil
}

// Unlock decrypts the given .enc file using AES-GCM decryption and removes the
// .enc extension.
func Unlock(ctx context.Context, path, passphrase string, logger *slog.Logger) error {
	log := logger.With(
		"cmp", "locker",
		"fn", "unlock",
		"file", filepath.Base(path),
	)

	log.Debug("unlocking file")

	if err := ctx.Err(); err != nil {
		log.Debug("unlock canceled", "error", err)
		return err
	}

	if err := validateInput(path, passphrase); err != nil {
		log.Warn("invalid unlock input", "error", err)
		return err
	}

	if !strings.HasSuffix(path, Extension.String()) {
		err := fmt.Errorf("%w: got %q, path: %q",
			ErrFileExtMismatch, filepath.Ext(path), path,
		)
		log.Warn("invalid encrypted file extension", "error", err)
		return err
	}

	// Read the encrypted content.
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		log.Error("failed to read encrypted file", "error", err)
		return fmt.Errorf("failed to read locked file: %w", err)
	}

	log.Debug("encrypted file read", "size", len(ciphertext))

	// Perform decryption.
	plaintext, err := Decrypt(ctx, ciphertext, passphrase)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Debug("decryption canceled", "error", ctxErr)
			return ctxErr
		}

		log.Warn("failed to decrypt file", "error", err)
		return err
	}

	log.Debug("file decrypted", "size", len(plaintext))

	// Create a backup before modifying files.
	backupPath, err := backupFile(log, path)
	if err != nil {
		log.Error("failed to create backup", "error", err)
		return fmt.Errorf("backup creation failed: %w", err)
	}

	log.Debug("backup created", "backup", filepath.Base(backupPath))

	// Write decrypted data to disk.
	decryptedPath := strings.TrimSuffix(path, Extension.String())

	if err := writeAndReplaceFile(log, decryptedPath, plaintext, path, backupPath); err != nil {
		log.Error("failed to replace encrypted file", "error", err)
		return err
	}

	log.Debug("file unlocked successfully")

	// Cleanup successful operation.
	if err := os.Remove(backupPath); err != nil {
		log.Warn("failed to remove temporary backup",
			"backup", filepath.Base(backupPath),
			"error", err,
		)
	}

	return nil
}

// IsLocked checks if the given file has .enc extension.
func IsLocked(path string) error {
	log := slog.Default().With(
		"cmp", "locker",
		"fn", "is-locked",
		"file", filepath.Base(path),
	)

	log.Debug("checking file lock state")

	encryptedPath := Extension.Join(path)
	if fileExists(encryptedPath) {
		log.Debug("file is locked")
		return ErrFileLocked
	}

	log.Debug("file is not locked")

	return nil
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

	if !fileExists(path) {
		return fmt.Errorf("%w: %s", os.ErrNotExist, path)
	}

	return nil
}

// backupFile creates a backup of the given file.
func backupFile(log *slog.Logger, filePath string) (string, error) {
	log = log.With(
		"stage", "backup-file",
		"file", filepath.Base(filePath),
	)

	log.Debug("creating file backup")

	if !fileExists(filePath) {
		err := fmt.Errorf("%w: %s", os.ErrNotExist, filePath)
		log.Warn("source file not found", "error", err)
		return "", err
	}

	// Generate backup filename with timestamp.
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup_%s", filePath, timestamp)

	// Read the original file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Error("failed to read file for backup", "error", err)
		return "", fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Write backup file.
	//nolint:gosec // false positive
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		log.Error("failed to create backup file", "error", err)
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}

	log.Debug("backup file created",
		"backup", filepath.Base(backupPath),
		"size", len(data),
	)

	return backupPath, nil
}

// writeAndReplaceFile writes data to targetPath, removes originalPath on
// success, and handles error recovery using backupPath.
func writeAndReplaceFile(log *slog.Logger, targetPath string, data []byte, originalPath, backupPath string) error {
	log = log.With(
		"stage", "write-and-replace-file",
		"target", filepath.Base(targetPath),
	)

	log.Debug("writing file", "size", len(data))

	// Write data to the target file.
	//nolint:gosec // false positive
	err := os.WriteFile(targetPath, data, 0o600)
	if err != nil {
		log.Error("failed to write target file", "error", err)

		log.Debug("restoring backup",
			"backup", filepath.Base(backupPath),
		)

		restoreErr := os.Rename(backupPath, originalPath)
		if restoreErr != nil {
			log.Error("failed to restore backup", "error", restoreErr)

			return fmt.Errorf(
				"failed to write file and restore backup: %w (original error: %w)",
				restoreErr,
				err,
			)
		}

		log.Debug("backup successfully restored")

		return fmt.Errorf("failed to write file: %w", err)
	}

	// Remove the original file.
	if filepath.Clean(targetPath) != filepath.Clean(originalPath) {
		log.Debug("removing original file", "original", filepath.Base(originalPath))

		if err := os.Remove(originalPath); err != nil {
			log.Error("failed to remove original file", "error", err)

			log.Debug("restoring backup",
				"backup", filepath.Base(backupPath),
			)

			restoreErr := os.Rename(backupPath, originalPath)
			if restoreErr != nil {
				log.Error("failed to restore backup", "error", restoreErr)

				return fmt.Errorf(
					"failed to remove original file and restore backup: %w (original error: %w)",
					restoreErr,
					err,
				)
			}

			log.Debug("backup successfully restored")

			return fmt.Errorf("failed to remove original file: %w", err)
		}
	}

	log.Debug("file written and replaced successfully")

	return nil
}

func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}
