package git

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

const (
	DirPerm  = 0o755 // Permissions for new directories.
	FilePerm = 0o644 // Permissions for new files.
)

// Exists checks if a file fileExists.
func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func writeFile[T any](path string, v *T) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

func readFile[T any](path string, v *T) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, &v)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	return nil
}

// genHash generates a hash from a string with the given length.
func genHash(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}

func which(cmd string) (string, error) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "", exec.ErrNotFound
	}
	return path, nil
}
