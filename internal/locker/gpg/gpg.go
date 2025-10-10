// Package gpg provides utilities for GPG encryption, decryption, and
// integration with Git.
package gpg

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/files"
)

var execCommand = exec.CommandContext

var (
	ErrKeyNotTrusted  = errors.New("GPG key is not trusted")
	ErrNoFingerprint  = errors.New("gpg: no fingerprint found")
	ErrNoGPGIDFile    = errors.New("gpg: no .gpg-id file found")
	ErrNoGPGRecipient = errors.New("gpg: no GPG recipient configured")
)

const (
	Command               = "gpg"
	gitAttContent         = "*.gpg diff=gpg"
	fingerprintIDFilename = ".gpg-id"
	Extension             = ".gpg"
)

// GitDiffConf is the gpg diff configuration for git.
var GitDiffConf = map[string][]string{
	"diff.gpg.binary": {"true"},
	"diff.gpg.textconv": {
		Command,
		"-d",
		"--quiet",
		"--yes",
		"--compress-algo=none",
		"--no-encrypt-to",
		"--batch",
		"--use-agent",
	},
}

// GPG holds configuration for running GPG commands.
type GPG struct {
	Recipient string
	BinPath   string
}

// Decrypt decrypts a file using the configured GPG binary.
func (g *GPG) Decrypt(ctx context.Context, encryptedPath string) ([]byte, error) {
	cmd := execCommand(ctx, g.BinPath, flags.quiet, flags.decrypt, encryptedPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		return nil, fmt.Errorf("gpg decrypt failed: %s: %w", msg, err)
	}

	return output, nil
}

// Encrypt encrypts data for the configured recipient and writes it to path.
func (g *GPG) Encrypt(ctx context.Context, path string, content []byte) error {
	if g.Recipient == "" {
		return ErrNoGPGRecipient
	}

	cmd := execCommand(
		ctx,
		g.BinPath,
		flags.yes,
		flags.encrypt,
		flags.recipient,
		g.Recipient,
		flags.output,
		path,
	)
	cmd.Stdin = bytes.NewReader(content)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg encrypt failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// New returns a new GPG instance after locating the gpg binary.
func New(recipient string) (*GPG, error) {
	binPath, err := sys.Which(Command)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, Command)
	}

	return &GPG{
		Recipient: recipient,
		BinPath:   binPath,
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

// Decrypt decrypts the provided encrypted file.
func Decrypt(fingerprintPath, encryptedPath string) ([]byte, error) {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return nil, err
	}

	g, err := New(recipientKey)
	if err != nil {
		return nil, err
	}

	return g.Decrypt(context.Background(), encryptedPath)
}

// Encrypt encrypts the provided data and saves it to the specified path.
func Encrypt(fingerprintPath, path string, content []byte) error {
	recipientKey, err := loadFingerprint(fingerprintPath)
	if err != nil {
		return err
	}

	g, err := New(recipientKey)
	if err != nil {
		return err
	}

	return g.Encrypt(context.Background(), path, content)
}

// loadFingerprint loads fingerprint from the .gpg-id file.
func loadFingerprint(f string) (string, error) {
	if !files.Exists(f) {
		return "", fmt.Errorf("%w: %q", ErrNoGPGIDFile, f)
	}

	fingerprint, err := os.ReadFile(f)
	if err != nil {
		return "", fmt.Errorf("failed to read .gpg-id: %w", err)
	}

	recipientKey := strings.TrimSpace(string(fingerprint))
	if recipientKey == "" {
		return "", ErrNoFingerprint
	}

	return recipientKey, nil
}

// execGPGListKeys executes the GPG command and returns its raw colon-delimited output.
func execGPGListKeys() ([]byte, error) {
	binPath, err := sys.Which(Command)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, Command)
	}

	cmd := exec.Command(
		binPath,
		flags.listKeys,
		flags.withColons,
		flags.fingerprint,
		flags.batch,
		flags.quiet,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gpg list-keys failed to execute: %w", err)
	}

	return output, nil
}

// parseGPGOutput processes the raw colon-delimited GPG output into Fingerprint structs.
func parseGPGOutput(output []byte) ([]*Fingerprint, error) {
	const (
		trustFieldIndex = 1
		keyIDFieldIndex = 4
		fprFieldIndex   = 9
		uidFieldIndex   = 9
	)
	var (
		keys    []*Fingerprint
		current *Fingerprint
		lastTag string
	)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) <= fprFieldIndex {
			continue
		}
		tag := fields[0]
		switch tag {
		case "pub":
			k := &Fingerprint{
				KeyID:        fields[keyIDFieldIndex],
				TrustLevel:   fields[trustFieldIndex], // Add this
				IsPrimaryKey: true,
			}
			keys = append(keys, k)
			current = keys[len(keys)-1]
			lastTag = tag
		case "uid":
			if current != nil && current.UserID == "" {
				current.UserID = strings.TrimSpace(fields[uidFieldIndex])
			}
			lastTag = tag
		case "fpr":
			fp := strings.TrimSpace(fields[fprFieldIndex])
			if current == nil {
				continue
			}
			switch lastTag {
			case "pub":
				current.Fingerprint = fp
			case "sub":
				current.Subkeys = append(current.Subkeys, fp)
			}
			lastTag = tag
		case "sub":
			if current != nil {
				current.SubkeyIDs = append(current.SubkeyIDs, fields[keyIDFieldIndex])
			}
			lastTag = tag
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error during parsing: %w", err)
	}
	return keys, nil
}

// ListFingerprints lists all public GPG keys with their fingerprints and subkeys.
func ListFingerprints() ([]*Fingerprint, error) {
	output, err := execGPGListKeys()
	if err != nil {
		return nil, err
	}

	fps, err := parseGPGOutput(output)
	if err != nil {
		return nil, err
	}

	if len(fps) == 0 {
		return nil, ErrNoFingerprint
	}

	return fps, nil
}

// Init will extract the gpg fingerprint and save it to .gpg-id.
func Init(path, gitAttrFile string, fingerprint *Fingerprint) error {
	if _, err := sys.Which(Command); err != nil {
		return fmt.Errorf("%w: %s", err, Command)
	}

	if err := files.MkdirAll(path); err != nil {
		return fmt.Errorf("%w", err)
	}

	fileIDPath := filepath.Join(path, fingerprintIDFilename)
	err := os.WriteFile(fileIDPath, []byte(fingerprint.Fingerprint+"\n"), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gpg-id: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, gitAttrFile), []byte(gitAttContent), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return nil
}

// GPGIDPath returns the path to the .gpg-id file inside the given repo directory.
func GPGIDPath(repoPath string) string {
	return filepath.Join(repoPath, ".gpg-id")
}
