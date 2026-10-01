package git

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	DirPerm  = 0o755 // Permissions for new directories.
	FilePerm = 0o644 // Permissions for new files.
)

type fileManager struct {
	rename       func(oldpath, newpath string) error
	remove       func(path string) error
	removeExcept func(dir string, keep map[string]struct{}) error
}

func newFileManager() *fileManager {
	return &fileManager{
		rename:       os.Rename,
		remove:       os.RemoveAll,
		removeExcept: removeAllExcept,
	}
}

type JSONFile[T any] struct {
	path       string
	value      T
	readFunc   func(path string, value *T) error
	writeFunc  func(path string, value *T) error
	existsFunc func(path string) bool
}

func newJSONFile[T any](path string) *JSONFile[T] {
	return &JSONFile[T]{
		path:       path,
		readFunc:   readFile[T],
		writeFunc:  writeFile[T],
		existsFunc: fileExists,
	}
}

func (f *JSONFile[T]) exists() bool { return f.existsFunc(f.path) }
func (f *JSONFile[T]) read() error  { return f.readFunc(f.path, &f.value) }
func (f *JSONFile[T]) set(v T)      { f.value = v }
func (f *JSONFile[T]) write() error { return f.writeFunc(f.path, &f.value) }

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

	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return err
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
		return fmt.Errorf("read file %q: %w", path, err)
	}
	if err := json.Unmarshal(content, v); err != nil {
		return fmt.Errorf("decode JSON from %q: %w", path, err)
	}
	return nil
}

func removeAllExcept(dir string, keep map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if _, ok := keep[entry.Name()]; ok {
			continue
		}

		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
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
