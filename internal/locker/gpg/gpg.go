package gpg

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	gitAttPath    = ".gitattributes"
	gitAttContent = "*.gpg diff=gpg"
	FilenameID    = ".gpg-id"
	cmdGPG        = "gpg"
)

//nolint:unused //notneeded
var gpgArgs = []string{
	"--quiet", "--yes", "--compress-algo=none", "--no-encrypt-to",
}

// IsActive returns true if GPG is active.
func IsActive(path string) bool {
	if err := loadFingerprint(path); err != nil {
		return false
	}
	return recipient != ""
}

// PromptGPGPassphrase tries to prompt for the passphrase using a dummy decryption.
func PromptGPGPassphrase() error {
	cmd := exec.Command(cmdGPG, "--quiet", "--batch", "--yes", "--list-secret-keys")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to prompt GPG passphrase: %w", err)
	}
	return nil
}

func Decrypt(encryptedPath string) ([]byte, error) {
	cmd := exec.Command(cmdGPG, "--quiet", "-d", encryptedPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		return nil, fmt.Errorf("gpg decrypt failed: %s: %w", msg, err)
	}
	return output, nil
}

func encrypt(path string, content []byte) error {
	cmd := exec.Command(cmdGPG, "--yes", "-e", "-r", recipient, "-o", path)
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
	cmd := exec.Command(cmdGPG, "--list-keys", "--with-colons")
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
	f := filepath.Join(path, FilenameID)
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

// Save encrypts the provided data and saves it to the specified path.
func Save(root, path string, b any) error {
	path = files.StripSuffixes(path) + ".gpg"
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	dir := filepath.Dir(path)
	if err := files.MkdirAll(dir); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return encrypt(path, data)
}

// Init will extract the gpg fingerprint and save it to .gpg-id.
func Init(path string) error {
	if err := sys.Which(cmdGPG); err != nil {
		return fmt.Errorf("%w: %s", err, cmdGPG)
	}
	if err := files.MkdirAll(path); err != nil {
		return fmt.Errorf("%w", err)
	}
	fileIDPath := filepath.Join(path, FilenameID)
	fingerprint, err := extractFingerPrint()
	if err != nil {
		return err
	}
	err = os.WriteFile(fileIDPath, []byte(fingerprint+"\n"), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gpg-id: %w", err)
	}
	err = os.WriteFile(filepath.Join(path, gitAttPath), []byte(gitAttContent), files.FilePerm)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return nil
}

// Create encrypts the provided data and saves it to the specified path.
func Create(root, hashPath string, bookmark any) error {
	if err := files.MkdirAll(root); err != nil {
		return fmt.Errorf("%w", err)
	}

	filePath := filepath.Join(root, hashPath+".gpg")
	if files.Exists(filePath) {
		return files.ErrFileExists
	}

	return Save(root, filePath, bookmark)
}
