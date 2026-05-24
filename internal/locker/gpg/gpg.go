// Package gpg provides utilities for GPG encryption, decryption, and
// integration with Git.
package gpg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrKeyNotTrusted  = errors.New("gpg: key is not trusted")
	ErrNoFingerprint  = errors.New("gpg: no fingerprint found")
	ErrNoGPGIDFile    = errors.New("gpg: no .gpg-id file found")
	ErrNoGPGRecipient = errors.New("gpg: no GPG recipient configured")
)

const (
	Command               = "gpg"            // Command is the GPG executable name.
	gitAttContent         = "*.gpg diff=gpg" // gitAttContent defines the Git attributes rule for encrypted files.
	fingerprintIDFilename = ".gpg-id"        // fingerprintIDFilename is the filename storing the GPG recipient fingerprint.
	Extension             = ".gpg"           // Extension is the file extension for encrypted files.
)

const (
	dirPerm  = 0o755 // Permissions for new directories.
	filePerm = 0o644 // Permissions for new files.
)

// GPG holds configuration for running GPG commands.
type GPG struct {
	recipient string
	binPath   string
	exec      func(context.Context, ...string) *exec.Cmd
}

// Decrypt decrypts a file using the configured GPG binary.
func (g *GPG) Decrypt(ctx context.Context, encryptedPath string) ([]byte, error) {
	slog.Debug("gpg: starting decryption")

	cmd := g.exec(
		ctx,
		flags.quiet,
		flags.decrypt,
		encryptedPath,
	)

	slog.Debug("gpg: executing GPG command", "args", cmd.Args)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		slog.Debug("gpg: decryption failed", "error", err, "output", msg, "output_length", len(output))
		return nil, fmt.Errorf("gpg decrypt failed: %s: %w", msg, err)
	}

	slog.Info("gpg: decryption successful", "encrypted_path", encryptedPath, "output_size", len(output))

	return output, nil
}

// Encrypt encrypts data for the configured recipient and writes it to path.
func (g *GPG) Encrypt(ctx context.Context, path string, content []byte) error {
	if g.recipient == "" {
		return ErrNoGPGRecipient
	}

	slog.Debug("gpg: starting encryption")

	cmd := g.exec(
		ctx,
		flags.yes,
		flags.encrypt,
		flags.recipient,
		g.recipient,
		flags.output,
		path,
	)

	slog.Debug("gpg: executing GPG command", "args", cmd.Args)

	cmd.Stdin = bytes.NewReader(content)

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("gpg: encryption failed", "error", err, "output_length", len(output))
		return fmt.Errorf("gpg encrypt failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	slog.Info("gpg: encryption successful", "encrypted_path", path, "output_size", len(output))

	return nil
}

// Unlocked reports whether the given encrypted file can be decrypted without a
// passphrase prompt.
func (g *GPG) Unlocked(ctx context.Context, filePath string) (bool, error) {
	slog.Debug("gpg: checking if unlocked")

	cmd := g.exec(
		ctx,
		flags.batch,
		flags.pinentryMode(ModeError),
		flags.decrypt,
		filePath,
	)

	slog.Debug("gpg: executing GPG command", "args", cmd.Args)

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	u := cmd.Run() == nil

	slog.Debug("gpg: result", "unlocked", u)

	return u, nil
}

// New returns a new GPG instance after locating the gpg binary.
func New(recipient string) (*GPG, error) {
	binPath, err := which()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, Command)
	}

	e := func(binPath string) func(context.Context, ...string) *exec.Cmd {
		return func(ctx context.Context, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, binPath, args...)
		}
	}

	return &GPG{
		recipient: recipient,
		binPath:   binPath,
		exec:      e(binPath),
	}, nil
}

// IsInitialized returns true if GPG is active.
func IsInitialized(path string) bool {
	f := GPGIDPath(path)
	recipientKey, err := loadFingerprint(f)
	if err != nil {
		return false
	}

	return recipientKey != ""
}

// Unlocked reports whether the given encrypted file can be decrypted without a
// passphrase prompt.
func Unlocked(ctx context.Context, filePath string) (bool, error) {
	g, err := New("")
	if err != nil {
		return false, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	return g.Unlocked(ctx, filePath)
}

// Decrypt decrypts the provided encrypted file.
func Decrypt(ctx context.Context, fingerprintPath, encryptedPath string) ([]byte, error) {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return nil, err
	}

	g, err := New(recipientKey)
	if err != nil {
		return nil, err
	}

	return g.Decrypt(ctx, encryptedPath)
}

// Encrypt encrypts the provided data and saves it to the specified path.
func Encrypt(ctx context.Context, fingerprintPath, path string, content []byte) error {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return err
	}

	g, err := New(recipientKey)
	if err != nil {
		return err
	}

	return g.Encrypt(ctx, path, content)
}

// Init will extract the gpg fingerprint and save it to .gpg-id.
func Init(path, gitAttrFile string, fingerprint *Fingerprint) error {
	if _, err := which(); err != nil {
		return fmt.Errorf("%w: %s", err, Command)
	}

	if err := os.MkdirAll(path, dirPerm); err != nil {
		return fmt.Errorf("%w", err)
	}

	fileIDPath := filepath.Join(path, fingerprintIDFilename)
	err := os.WriteFile(fileIDPath, []byte(fingerprint.Fingerprint+"\n"), filePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gpg-id: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, gitAttrFile), []byte(gitAttContent), filePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return nil
}

// GPGIDPath returns the path to the .gpg-id file inside the given repo directory.
func GPGIDPath(repoPath string) string {
	return filepath.Join(repoPath, ".gpg-id")
}

func which() (string, error) {
	path, err := exec.LookPath(Command)
	if err != nil {
		return "", exec.ErrNotFound
	}
	return path, nil
}

func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}
