package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrPathNotFound = errors.New("path not found")
)

// FilesWithSuffix finds all files with a specific suffix within a directory.
func FilesWithSuffix(path, suffix string, files *[]string) error {
	if !FileExists(path) {
		return ErrPathNotFound
	}

	var err error
	*files, err = filepath.Glob(path + "/*." + suffix)
	if err != nil {
		return fmt.Errorf("getting files: %w with suffix: '%s'", err, suffix)
	}

	return nil
}

// FileExists checks if a file exists.
func FileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// Filesize returns the size of a file.
func Filesize(f string) int64 {
	fi, err := os.Stat(f)
	if err != nil {
		return 0
	}

	return fi.Size()
}

// Files returns files found in a given path.
func Files(path, name string) ([]string, error) {
	query := path + "/*" + name
	files, err := filepath.Glob(query)
	if err != nil {
		return nil, fmt.Errorf("%w: getting files query: '%s'", err, query)
	}

	return files, nil
}

func Mkdir(path string) error {
	if FileExists(path) {
		return nil
	}

	const dirPermissions = 0o755
	log.Printf("creating path: '%s'", path)
	if err := os.MkdirAll(path, dirPermissions); err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	return nil
}

func RmFile(f string) error {
	if !FileExists(f) {
		return fmt.Errorf("%w: '%s'", ErrFileNotFound, f)
	}
	if err := os.Remove(f); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}

// CopyFile copies a file.
func CopyFile(src, dst string) error {
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

// SortFilesByMod sorts files by modification time.
func SortFilesByMod(f []fs.DirEntry) {
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

// CleanupTempFile Removes the specified temporary file.
func CleanupTempFile(fileName string) error {
	err := os.Remove(fileName)
	if err != nil {
		return fmt.Errorf("could not cleanup temp file: %w", err)
	}

	return nil
}

// CreateTempFile Creates a temporary file with the provided prefix.
func CreateTempFile(prefix, ext string) (*os.File, error) {
	filename := fmt.Sprintf("%s-*.%s", prefix, ext)
	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	return tempFile, nil
}
