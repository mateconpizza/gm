package bookio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrFileNotFound = errors.New("file not found")

const (
	jsonExt         = ".json"
	summaryFile     = "summary.json"  // Git and database metadata
	trackerFilepath = ".tracked.json" // Tracked databases in Git
)

var JSONStrategy = &RepositoryLoader{
	LoaderFn: jsonLoader,
	Prefix:   "Loading JSON bookmarks",
	FileFilter: And(
		IsFile,
		HasExtension(jsonExt),
		NotNamed(summaryFile, trackerFilepath),
	),
}

func jsonLoader(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	bj := &bookmark.BookmarkJSON{}
	if err := files.JSONRead(path, bj); err != nil {
		return nil, fmt.Errorf("%w: %s", err, path)
	}

	return bookmark.NewFromJSON(bj), nil
}

// storeBookmarkAsJSON creates files structure.
//
//	root -> dbName -> domain -> urlHash.json
//
// Returns true if the file was created or updated, false if no changes were made.
func storeBookmarkAsJSON(rootPath string, b *bookmark.Bookmark, force bool) (bool, error) {
	domain, err := b.Domain()
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// domainPath: root -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// urlHash := domainPath -> urlHash.json
	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+jsonExt)

	updated, err := files.JSONWrite(filePathJSON, b.JSON(), force)
	if err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return updated, nil
}

// resolveFileConflictErr resolves a file conflict error.
// Returns true if the file was updated, false if no update was needed.
func resolveFileConflictErr(
	rootPath string,
	err error,
	filePathJSON string,
	b *bookmark.Bookmark,
) (bool, error) {
	if !errors.Is(err, files.ErrFileExists) {
		return false, err
	}

	bj := bookmark.BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// no need to update
	if bj.Checksum == b.Checksum {
		return false, nil
	}

	return storeBookmarkAsJSON(rootPath, b, true)
}
