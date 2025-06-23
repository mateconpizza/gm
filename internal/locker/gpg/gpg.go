package gpg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
)

var (
	ErrNoFingerprint = errors.New("no fingerprint found")
	ErrNoGPGIDFile   = errors.New("no .gpg-id file found")
)

var recipient string

const (
	gitAttContent = "*.gpg diff=gpg"
	FingerprintID = ".gpg-id"
	gpgCommand    = "gpg"
	Extension     = ".gpg"
)

// GitDiffConf is the gpg diff configuration for git.
var GitDiffConf = map[string][]string{
	"diff.gpg.binary": {"true"},
	"diff.gpg.textconv": {
		gpgCommand,
		"-d",
		"--quiet",
		"--yes",
		"--compress-algo=none",
		"--no-encrypt-to",
		"--batch",
		"--use-agent",
	},
}

// IsInitialized returns true if GPG is active.
func IsInitialized(path string) bool {
	if err := loadFingerprint(path); err != nil {
		return false
	}

	return recipient != ""
}

// Decrypt decrypts the provided encrypted file.
func Decrypt(encryptedPath string) ([]byte, error) {
	cmdPath, err := sys.Which(gpgCommand)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, gpgCommand)
	}

	cmd := exec.Command(cmdPath, "--quiet", "-d", encryptedPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		return nil, fmt.Errorf("gpg decrypt failed: %s: %w", msg, err)
	}

	return output, nil
}

// Encrypt encrypts the provided data and saves it to the specified path.
func Encrypt(path string, content []byte) error {
	cmdPath, err := sys.Which(gpgCommand)
	if err != nil {
		return fmt.Errorf("%w: %s", err, cmdPath)
	}

	cmd := exec.Command(cmdPath, "--yes", "-e", "-r", recipient, "-o", path)
	cmd.Stdin = bytes.NewReader(content)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(strings.TrimSpace(string(output)))
		return fmt.Errorf("%w", err)
	}

	return nil
}

// extractFingerPrint will extract the gpg fingerprint from the output of `gpg
// --list-keys --with-colons`.
func extractFingerPrint() (string, error) {
	cmdPath, err := sys.Which(gpgCommand)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, cmdPath)
	}

	cmd := exec.Command(cmdPath, "--list-keys", "--with-colons")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gpg list-keys: %w", err)
	}

	const fingerprintFieldIndex = 9

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "fpr:") {
			fields := strings.Split(line, ":")
			if len(fields) > fingerprintFieldIndex {
				fingerprint := fields[fingerprintFieldIndex]
				return fingerprint, nil
			}
		}
	}

	return "", ErrNoFingerprint
}

// loadFingerprint loads fingerprint from the .gpg-id file.
func loadFingerprint(path string) error {
	f := filepath.Join(path, FingerprintID)
	if !files.Exists(f) {
		return ErrNoGPGIDFile
	}

	fingerprint, err := os.ReadFile(f)
	if err != nil {
		return fmt.Errorf("failed to read .gpg-id: %w", err)
	}

	recipient = strings.TrimSpace(string(fingerprint))
	if recipient == "" {
		return ErrNoFingerprint
	}

	return nil
}

// Init will extract the gpg fingerprint and save it to .gpg-id.
func Init(path, gitAttrFile string) error {
	if _, err := sys.Which(gpgCommand); err != nil {
		return fmt.Errorf("%w: %s", err, gpgCommand)
	}

	if err := files.MkdirAll(path); err != nil {
		return fmt.Errorf("%w", err)
	}

	fileIDPath := filepath.Join(path, FingerprintID)

	fingerprint, err := extractFingerPrint()
	if err != nil {
		return err
	}

	err = os.WriteFile(fileIDPath, []byte(fingerprint+"\n"), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gpg-id: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, gitAttrFile), []byte(gitAttContent), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return nil
}
