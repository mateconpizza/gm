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
	ErrNoFingerprint  = errors.New("gpg: no fingerprint found")
	ErrNoGPGIDFile    = errors.New("gpg: no .gpg-id file found")
	ErrNoGPGRecipient = errors.New("gpg: no GPG recipient configured")
)

type Ext string

func (e Ext) Join(filename string) string {
	if strings.HasSuffix(filename, e.String()) {
		return filename
	}
	return filename + e.String()
}

func (e Ext) String() string { return string(e) }

const (
	Command       = "gpg"  // Command is the GPG executable name.
	Extension Ext = ".gpg" // Extension is the file extension for encrypted files.
)

const (
	dirPerm  = 0o700 // Permissions for new directories.
	filePerm = 0o600 // Permissions for new files.
)

type (
	// GPG holds configuration for running GPG commands.
	GPG struct {
		recipient string
		exec      Executor

		logger *slog.Logger
	}

	Executor func(ctx context.Context, args ...string) *exec.Cmd

	GPGOpt func(*GPG)
)

// New returns a new GPG instance after locating the gpg binary.
func New(recipient string, opts ...GPGOpt) *GPG {
	g := &GPG{
		recipient: recipient,
		exec:      defaultExecuter(Command),
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func WithExecutor(fn Executor) GPGOpt  { return func(g *GPG) { g.exec = fn } }
func WithLogger(l *slog.Logger) GPGOpt { return func(g *GPG) { g.logger = l } }

// Decrypt decrypts a file using the configured GPG binary.
func (g *GPG) Decrypt(ctx context.Context, encryptedPath string) ([]byte, error) {
	cmd := g.exec(
		ctx,
		flags.quiet,
		flags.decrypt,
		encryptedPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("gpg decrypt failed: %w: %s", err, strings.TrimSuffix(stderr.String(), "\n"))
	}

	output := stdout.Bytes()

	return output, nil
}

// Encrypt encrypts data for the configured recipient and writes it to path.
func (g *GPG) Encrypt(ctx context.Context, path string, content []byte) error {
	if g.recipient == "" {
		return ErrNoGPGRecipient
	}

	log := g.logger.With("fn", "gpg-encrypt", "path", path)
	log.Debug("starting encryption", "recipient", g.recipient)

	cmd := g.exec(
		ctx,
		flags.yes,
		flags.encrypt,
		flags.recipient,
		g.recipient,
		flags.output,
		path,
	)

	log.Debug("executing command", "args", cmd.Args)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = bytes.NewReader(content)

	err := cmd.Run()
	if err != nil {
		if strings.Contains(stderr.String(), "Unusable public key") {
			return fmt.Errorf("%w: %q", ErrKeyUnusable, g.recipient)
		}

		return fmt.Errorf("gpg encrypt failed: %w: %s", err, stderr.String())
	}

	log.Info("encryption successful")

	return nil
}

// Unlocked reports whether the given encrypted file can be decrypted without a
// passphrase prompt.
func (g *GPG) Unlocked(ctx context.Context, filePath string) (bool, error) {
	log := g.logger.With("fn", "gpg-unlocked", "path", filePath)

	log.Debug("checking if unlocked")

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

// IsInitialized returns true if GPG is active.
func IsInitialized(path string) bool { return NewKeyResolver(path).Initialized() }

// Unlocked reports whether the given encrypted file can be decrypted without a
// passphrase prompt.
func Unlocked(ctx context.Context, filePath string) (bool, error) {
	return New("").Unlocked(ctx, filePath)
}

// Decrypt decrypts the provided encrypted file.
func Decrypt(ctx context.Context, fingerprintPath, encryptedPath string) ([]byte, error) {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return nil, err
	}
	return New(recipientKey).Decrypt(ctx, encryptedPath)
}

// Encrypt encrypts the provided data and saves it to the specified path.
func Encrypt(ctx context.Context, fingerprintPath, path string, content []byte) error {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return err
	}
	return New(recipientKey).Encrypt(ctx, path, content)
}

// GPGIDPath returns the path to the .gpg-id file inside the given repo directory.
func GPGIDPath(repoPath string) string { return filepath.Join(repoPath, FingerprintIDFilename) }

func defaultExecuter(binPath string) func(context.Context, ...string) *exec.Cmd {
	return func(ctx context.Context, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, binPath, args...)
	}
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
