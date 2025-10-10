package bookio

import (
	"io/fs"
	"path/filepath"
	"slices"
)

type FileFilterFunc func(path string, d fs.DirEntry) bool

// Common filters.
var (
	// IsFile returns true if the entry is a regular file (not a directory).
	IsFile = func(path string, d fs.DirEntry) bool {
		return !d.IsDir()
	}

	// HasExtension returns a filter that matches files with the given extension.
	HasExtension = func(ext string) FileFilterFunc {
		return func(path string, d fs.DirEntry) bool {
			return filepath.Ext(path) == ext
		}
	}

	// NotNamed returns a filter that excludes files with the specified names.
	NotNamed = func(names ...string) FileFilterFunc {
		return func(path string, d fs.DirEntry) bool {
			base := filepath.Base(path)
			return !slices.Contains(names, base)
		}
	}
)

// And returns a filter that matches when all given filters match.
func And(filters ...FileFilterFunc) FileFilterFunc {
	return func(path string, d fs.DirEntry) bool {
		for _, f := range filters {
			if !f(path, d) {
				return false
			}
		}

		return true
	}
}

// Or returns a filter that matches when any given filter matches.
func Or(filters ...FileFilterFunc) FileFilterFunc {
	return func(path string, d fs.DirEntry) bool {
		for _, f := range filters {
			if f(path, d) {
				return true
			}
		}

		return false
	}
}

// Not returns a filter that inverts the result of the given filter.
func Not(filter FileFilterFunc) FileFilterFunc {
	return func(path string, d fs.DirEntry) bool {
		return !filter(path, d)
	}
}
