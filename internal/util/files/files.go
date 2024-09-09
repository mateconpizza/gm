// Package files provides utilities for working with files
package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrPathNotFound = errors.New("path not found")
)

// Exists checks if a file exists.
func Exists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// Size returns the size of a file.
func Size(f string) int64 {
	fi, err := os.Stat(f)
	if err != nil {
		return 0
	}

	return fi.Size()
}

// GetLastFile returns the last file found in a given path.
func GetLastFile(path, name string) (string, error) {
	files, err := List(path, name)
	if err != nil {
		return "", fmt.Errorf("%w: getting files from '%s'", err, path)
	}

	return files[len(files)-1], nil
}

// List returns files found in a given path.
func List(path, target string) ([]string, error) {
	query := path + "/*" + target
	files, err := filepath.Glob(query)
	if err != nil {
		return nil, fmt.Errorf("%w: getting files query: '%s'", err, query)
	}

	log.Printf("found %d files in path: '%s'", len(files), path)

	return files, nil
}

// Mkdir creates a new directory at the specified path if it does not already
// exist.
func Mkdir(path string) error {
	if Exists(path) {
		return nil
	}

	const dirPermissions = 0o755
	log.Printf("creating path: '%s'", path)
	if err := os.MkdirAll(path, dirPermissions); err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	return nil
}

// Remove removes the specified file if it exists, returning an error if the
// file does not exist or if removal fails.
func Remove(f string) error {
	if !Exists(f) {
		return fmt.Errorf("%w: '%s'", ErrFileNotFound, f)
	}

	log.Printf("removing file: '%s'", f)

	if err := os.Remove(f); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}

// Copy copies the contents of a source file to a destination file,
// returning an error if any file operation fails.
func Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}

	defer func() {
		if err := srcFile.Close(); err != nil {
			log.Printf("error closing source file: %v", err)
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}

	log.Printf("copying '%s' to '%s'", filepath.Base(src), filepath.Base(dst))

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

// SortByMod sorts a slice of `fs.DirEntry` by the modification time of the
// files in ascending order.
func SortByMod(f []fs.DirEntry) {
	sort.Slice(f, func(i, j int) bool {
		fileI, err := f[i].Info()
		if err != nil {
			return false
		}
		fileJ, err := f[j].Info()
		if err != nil {
			return false
		}

		return fileI.ModTime().Before(fileJ.ModTime())
	})
}

// cleanupTemp Removes the specified temporary file.
func cleanupTemp(fileName string) error {
	log.Printf("removing temp file: '%s'", fileName)
	err := os.Remove(fileName)
	if err != nil {
		return fmt.Errorf("could not cleanup temp file: %w", err)
	}

	return nil
}

// Cleanup closes the provided file and deletes the associated temporary file,
// logging any errors encountered.
func Cleanup(tf *os.File) {
	if err := tf.Close(); err != nil {
		log.Printf("Error closing temp file: %v", err)
	}
	if err := cleanupTemp(tf.Name()); err != nil {
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

// FindByExtension returns a list of files with the specified extension in
// the given directory, or an error if the directory does not exist or if the
// glob operation fails.
func FindByExtension(path, ext string) ([]string, error) {
	if !Exists(path) {
		return nil, ErrPathNotFound
	}

	files, err := filepath.Glob(path + "/*." + ext)
	if err != nil {
		return nil, fmt.Errorf("getting files: %w with suffix: '%s'", err, ext)
	}

	log.Printf("found %d files in path: '%s'", len(files), path)

	return files, nil
}

// EnsureExtension appends the specified suffix to the filename if it does
// not already have it.
func EnsureExtension(name, ext string) string {
	if !strings.HasSuffix(name, ext) {
		name = fmt.Sprintf("%s%s", name, ext)
	}

	return name
}
