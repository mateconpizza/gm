package bookio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

const (
	jsonExt         = ".json"
	summaryFile     = "summary.json"  // Git and database metadata
	trackerFilepath = ".tracked.json" // Tracked databases in Git
)

var JSONStrategy = &RepositoryLoader{
	Func:   jsonLoader,
	Prefix: "Loading JSON bookmarks",
	FileFilter: And(
		IsFile,
		HasExtension(jsonExt),
		NotNamed(summaryFile, trackerFilepath),
	),
}

func jsonLoader(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	bj := &bookmark.BookmarkJSON{}
	if err := files.ReadJSON(path, bj); err != nil {
		return nil, fmt.Errorf("%w: %s", err, path)
	}

	return bookmark.NewFromJSON(bj), nil
}

// SaveAsJSON creates files structure.
//
//	root -> dbName -> domain -> urlHash.json
//
// Returns true if the file was created or updated, false if no changes were made.
func SaveAsJSON(rootPath string, b *bookmark.Bookmark, force bool) (bool, error) {
	domain, err := b.Domain()
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+jsonExt)
	updated, err := files.WriteJSONIfChanged(filePathJSON, b.JSON(), force)

	// Handle file conflict
	if errors.Is(err, os.ErrExist) {
		bj := bookmark.BookmarkJSON{}
		if err := files.ReadJSON(filePathJSON, &bj); err != nil {
			return false, fmt.Errorf("%w", err)
		}

		// No need to update if checksums match
		if bj.Checksum == b.Checksum {
			return false, nil
		}

		// Checksums differ, force update
		return SaveAsJSON(rootPath, b, true)
	}

	if err != nil {
		return false, err
	}

	return updated, nil
}
