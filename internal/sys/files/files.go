// Package files provides utilities for working with files/directories.
package files

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrPathNotFound    = errors.New("path not found")
	ErrFileExists      = errors.New("file already exists")
	ErrNotFile         = errors.New("not a file")
	ErrPathEmpty       = errors.New("path is empty")
	ErrNothingToRemove = errors.New("nothing to remove")
)

// Exists checks if a file exists.
func Exists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// IsFile checks if the given path exists and refers to a regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}

		return false
	}

	return info.Mode().IsRegular()
}

// size returns the size of a file.
func size(s string) int64 {
	fi, err := os.Stat(s)
	if err != nil {
		return 0
	}

	return fi.Size()
}

// List returns all files found in a given path.
func List(root, pattern string) ([]string, error) {
	query := root + "/*" + pattern
	files, err := filepath.Glob(query)
	if err != nil {
		return nil, fmt.Errorf("%w: getting files query: %q", err, query)
	}

	slog.Debug("found files", "count", len(files), "path", root)

	return files, nil
}

// mkdir creates a new directory at the specified path.
func mkdir(s string) error {
	if Exists(s) {
		return nil
	}

	slog.Debug("creating path", "path", s)
	if err := os.MkdirAll(s, dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", s, err)
	}

	return nil
}

// MkdirAll creates all the given paths.
func MkdirAll(s ...string) error {
	for _, p := range s {
		if p == "" {
			return ErrPathEmpty
		}
		if err := mkdir(p); err != nil {
			return err
		}
	}

	return nil
}

// Remove removes the specified file if it exists.
func Remove(s string) error {
	if !Exists(s) {
		return fmt.Errorf("%w: %q", ErrFileNotFound, s)
	}

	slog.Debug("removing file", "path", s)

	if err := os.Remove(s); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}

// Copy copies the contents of a source file to a destination file.
func Copy(from, to string) error {
	srcFile, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}

	defer func() {
		if err := srcFile.Close(); err != nil {
			slog.Error("closing source file", "file", from, "error", err)
		}
	}()

	dstFile, err := Touch(to, false)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}

	slog.Debug("copying file", "from", from, "to", to)

	defer func() {
		if err := dstFile.Close(); err != nil {
			slog.Error("closing destination file", "file", dstFile.Name(), "error", err)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}

	return nil
}

// cleanupTemp Removes the specified temporary file.
func cleanupTemp(s string) error {
	slog.Debug("removing temp file", "file", s)
	err := os.Remove(s)
	if err != nil {
		return fmt.Errorf("could not cleanup temp file: %w", err)
	}

	return nil
}

// closeAndClean closes the provided file and deletes the associated temporary file.
func closeAndClean(f *os.File) {
	if err := f.Close(); err != nil {
		slog.Error("closing temp file", "file", f.Name(), "error", err)
	}
	if err := cleanupTemp(f.Name()); err != nil {
		slog.Error("removing temp file", "file", f.Name(), "error", err)
	}
}

// CreateTemp Creates a temporary file with the provided prefix.
func CreateTemp(prefix, ext string) (*os.File, error) {
	fileName := fmt.Sprintf("%s-*.%s", prefix, ext)
	slog.Debug("creating temp file", "name", fileName)
	tempFile, err := os.CreateTemp("", fileName)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	return tempFile, nil
}

func FindByExtList(root string, ext ...string) ([]string, error) {
	if !Exists(root) {
		slog.Warn("path not found", "path", root)
		return nil, ErrPathNotFound
	}

	var files []string
	for _, e := range ext {
		f, err := findByExt(root, e)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		files = append(files, f...)
	}

	return files, nil
}

// findByExt returns a list of files with the specified extension in the
// given directory.
func findByExt(root, ext string) ([]string, error) {
	if !Exists(root) {
		slog.Error("path not found", "path", root)
		return nil, ErrPathNotFound
	}

	files, err := filepath.Glob(root + "/*" + ext)
	if err != nil {
		return nil, fmt.Errorf("getting files: %w with suffix: %q", err, ext)
	}

	return files, nil
}

// EnsureExt appends the specified suffix to the filename.
func EnsureExt(s, suffix string) string {
	if s == "" {
		return s
	}
	e := filepath.Ext(s)
	if e == suffix || e != "" {
		return s
	}

	return fmt.Sprintf("%s%s", s, suffix)
}

// Empty returns true if the file at path s has non-zero size.
func Empty(s string) bool {
	return size(s) == 0
}

// ModTime returns the formatted modification time of the specified file.
func ModTime(s, format string) string {
	file, err := os.Stat(s)
	if err != nil {
		slog.Error("getting modification time", "file", s, "error", err)
		return ""
	}

	return file.ModTime().Format(format)
}

// Touch creates a file at this given path.
// If the file already exists, the function succeeds when exist_ok is true.
func Touch(s string, existsOK bool) (*os.File, error) {
	if Exists(s) && !existsOK {
		return nil, fmt.Errorf("%w: %q", ErrFileExists, s)
	}

	f, err := os.Create(s)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	return f, nil
}

func ExpandHomeDir(s string) string {
	if strings.HasPrefix(s, "~/") {
		dirname, _ := os.UserHomeDir()
		s = filepath.Join(dirname, s[2:])
	}

	return s
}

// YamlWrite writes the provided YAML data to the specified file.
func YamlWrite[T any](p string, v *T, force bool) error {
	f, err := Touch(p, force)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Yaml closing file", "file", p, "error", err)
		}
	}()

	data, err := yaml.Marshal(&v)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	slog.Info("YamlWrite success", "path", p)

	return nil
}

// YamlRead unmarshals the YAML data from the specified file.
func YamlRead[T any](p string, v *T) error {
	if !Exists(p) {
		return fmt.Errorf("%w: %q", ErrFileNotFound, p)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = yaml.Unmarshal(content, &v)
	if err != nil {
		return fmt.Errorf("error unmarshalling YAML: %w", err)
	}

	slog.Debug("YamlRead", "path", p)

	return nil
}

func Find(root, pattern string) ([]string, error) {
	f, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if len(f) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrFileNotFound, pattern)
	}

	return f, nil
}
