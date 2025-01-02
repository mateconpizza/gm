// Package files provides utilities for working with files/directories.
package files

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/haaag/gm/internal/config"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrPathNotFound = errors.New("path not found")
	ErrFileExists   = errors.New("file already exists")
)

// Exists checks if a file exists.
func Exists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
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
		return nil, fmt.Errorf("%w: getting files query: '%s'", err, query)
	}

	log.Printf("found %d files in path: '%s'", len(files), root)

	return files, nil
}

// mkdir creates a new directory at the specified path.
func mkdir(s string) error {
	if Exists(s) {
		return nil
	}

	log.Printf("creating path: '%s'", s)
	if err := os.MkdirAll(s, config.Permissions.Dir); err != nil {
		return fmt.Errorf("creating %s: %w", s, err)
	}

	return nil
}

// MkdirAll creates all the given paths.
func MkdirAll(s ...string) error {
	for _, path := range s {
		if err := mkdir(path); err != nil {
			return err
		}
	}

	return nil
}

// Remove removes the specified file if it exists.
func Remove(s string) error {
	if !Exists(s) {
		return fmt.Errorf("%w: '%s'", ErrFileNotFound, s)
	}

	log.Printf("removing file: '%s'", s)

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
			log.Printf("error closing source file: %v", err)
		}
	}()

	dstFile, err := Touch(to, false)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}

	log.Printf("copying '%s' to '%s'", filepath.Base(from), filepath.Base(to))

	defer func() {
		if err := dstFile.Close(); err != nil {
			log.Printf("error closing destination file: %v", err)
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
	log.Printf("removing temp file: '%s'", s)
	err := os.Remove(s)
	if err != nil {
		return fmt.Errorf("could not cleanup temp file: %w", err)
	}

	return nil
}

// closeAndClean closes the provided file and deletes the associated temporary file.
func closeAndClean(f *os.File) {
	if err := f.Close(); err != nil {
		log.Printf("Error closing temp file: %v", err)
	}
	if err := cleanupTemp(f.Name()); err != nil {
		log.Printf("%v", err)
	}
}

// CreateTemp Creates a temporary file with the provided prefix.
func CreateTemp(prefix, ext string) (*os.File, error) {
	fileName := fmt.Sprintf("%s-*.%s", prefix, ext)
	log.Printf("creating temp file: '%s'", fileName)
	tempFile, err := os.CreateTemp("", fileName)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	return tempFile, nil
}

func FindByExtList(root string, ext ...string) ([]string, error) {
	if !Exists(root) {
		log.Printf("FindByExtList: path does not exist: '%s'", root)
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
		log.Printf("FindByExtension: path does not exist: '%s'", root)
		return nil, ErrPathNotFound
	}

	log.Printf("searching files in path: '%s' with suffix: '%s'", root, ext)

	files, err := filepath.Glob(root + "/*" + ext)
	if err != nil {
		return nil, fmt.Errorf("getting files: %w with suffix: '%s'", err, ext)
	}

	log.Printf("found %d files", len(files))

	return files, nil
}

// EnsureExt appends the specified suffix to the filename.
func EnsureExt(s, suffix string) string {
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
		log.Printf("GetModTime: %v", err)
		return ""
	}

	return file.ModTime().Format(format)
}

// Touch creates a file at this given path.
// If the file already exists, the function succeeds when exist_ok is true.
func Touch(s string, exist_ok bool) (*os.File, error) {
	if Exists(s) && !exist_ok {
		return nil, fmt.Errorf("%w: '%s'", ErrFileExists, s)
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
