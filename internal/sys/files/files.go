// Package files provides utilities for working with files/directories.
package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

func ExistsErr(p string) error {
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}

		return fmt.Errorf("%w", err)
	}

	return nil
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
	slog.Debug("removing path", "path", s)
	if err := os.Remove(s); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}

// RemoveAll removes the specified file if it exists.
func RemoveAll(s string) error {
	if !Exists(s) {
		return fmt.Errorf("%w: %q", ErrFileNotFound, s)
	}
	slog.Debug("removing path", "path", s)
	if err := os.RemoveAll(s); err != nil {
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

// EnsureSuffix appends the specified suffix to the filename.
func EnsureSuffix(s, suffix string) string {
	if s == "" {
		return s
	}
	e := filepath.Ext(s)
	if e == suffix || e != "" {
		return s
	}

	return fmt.Sprintf("%s%s", s, suffix)
}

// StripSuffixes removes all suffixes from the path.
func StripSuffixes(p string) string {
	for ext := filepath.Ext(p); ext != ""; ext = filepath.Ext(p) {
		p = p[:len(p)-len(ext)]
	}

	return p
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

	if !Exists(filepath.Dir(s)) {
		if err := MkdirAll(filepath.Dir(s)); err != nil {
			return nil, err
		}
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

// JSONWrite writes the provided data as JSON to the specified file.
// It uses generics to accept any type `T`.
func JSONWrite[T any](p string, v *T, force bool) error {
	f, err := Touch(p, force)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Json closing file", "file", p, "error", err)
		}
	}()

	// Marshal the data to JSON.
	data, err := json.MarshalIndent(v, "", "  ") // Use MarshalIndent for pretty-printed JSON
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	slog.Info("JsonWrite success", "path", p)

	return nil
}

func JSONRead[T any](p string, v *T) error {
	if !Exists(p) {
		return fmt.Errorf("%w: %q", ErrFileNotFound, p)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = json.Unmarshal(content, &v)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	slog.Debug("JsonRead", "path", p)

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

func ListRootFolders(root string, ignored ...string) ([]string, error) {
	// FIX: return fullpath.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("listing root folders: %w", err)
	}

	var folders []string
	for _, entry := range entries {
		if entry.IsDir() && !slices.Contains(ignored, entry.Name()) {
			folders = append(folders, entry.Name())
		}
	}

	return folders, nil
}

// RemoveFilepath removes the file and its parent directory if empty.
func RemoveFilepath(fname string) error {
	if !Exists(fname) {
		slog.Debug("file not found", "path", fname)
		return fmt.Errorf("%w: %q", ErrFileNotFound, fname)
	}
	if err := Remove(fname); err != nil {
		return fmt.Errorf("removing file:%w", err)
	}
	// check if the directory is empty
	fdir := filepath.Dir(fname)
	dirs, err := List(fdir, "*")
	if err != nil {
		return fmt.Errorf("listing directory: %w", err)
	}
	if len(dirs) == 0 {
		// remove empty path
		if err := Remove(fdir); err != nil {
			return fmt.Errorf("removing directory: %w", err)
		}
	}

	return nil
}
