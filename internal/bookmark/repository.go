package bookmark

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
)

var ErrInvalidChecksum = errors.New("invalid checksum")

// ExportBookmarks creates the repository structure.
func ExportBookmarks(root, dbName string, bs []*Bookmark) error {
	dbName = strings.TrimSuffix(dbName, filepath.Ext(dbName))
	for _, b := range bs {
		if err := storeBookmarkJSON(root, dbName, b, config.App.Force); err != nil {
			return err
		}
	}

	return nil
}

// storeBookmarkJSON creates files structure.
func storeBookmarkJSON(rootPath, dbName string, b *Bookmark, force bool) error {
	domain, err := HashDomain(b.URL)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// domainPath: root -> data -> dbName -> domain
	domainPath := filepath.Join(rootPath, "data", dbName, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return fmt.Errorf("%w", err)
	}
	// urlHash := domainPath -> urlHash.json
	urlHash := HashURL(b.URL)
	filePathJSON := filepath.Join(domainPath, urlHash+".json")
	if err := files.JSONWrite(filePathJSON, b.ToJSON(), force); err != nil {
		return resolveFileConflictError(err, filePathJSON, b)
	}

	return nil
}

// resolveFileConflictError resolves a file conflict error.
func resolveFileConflictError(err error, filePathJSON string, b *Bookmark) error {
	if !errors.Is(err, files.ErrFileExists) {
		return err
	}
	bj := BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return fmt.Errorf("%w", err)
	}
	// no need to update
	if bj.Checksum == b.Checksum {
		return nil
	}
	s := strings.TrimSuffix(config.App.DBName, filepath.Ext(config.App.DBName))
	return storeBookmarkJSON(config.App.Path.Git, s, b, true)
}

// CleanupFiles removes the files associated with a bookmark.
func CleanupFiles(root, rawURL string) error {
	domain, err := HashDomain(rawURL)
	if err != nil {
		return fmt.Errorf("hasher domain: %w", err)
	}
	domainPath := filepath.Join(root, domain)
	urlHash := HashURL(rawURL)
	filePathJSON := filepath.Join(domainPath, urlHash+".json")
	if err := files.Remove(filePathJSON); err != nil {
		return fmt.Errorf("removing file:%w", err)
	}
	// check if the directory is empty
	dirs, err := files.List(domainPath, "*")
	if err != nil {
		return fmt.Errorf("listing directory: %w", err)
	}
	if len(dirs) == 0 {
		if err := files.Remove(domainPath); err != nil {
			return fmt.Errorf("removing directory: %w", err)
		}
	}
	return nil
}

func validateChecksum(b *BookmarkJSON) bool {
	tags := ParseTags(strings.Join(b.Tags, ","))
	return b.Checksum == Checksum(b.URL, b.Title, b.Desc, tags)
}

// errorTracker provides thread-safe error tracking.
type errorTracker struct {
	mu    sync.Mutex
	error error
}

// newErrorTracker creates a new error tracker.
func newErrorTracker() *errorTracker {
	return &errorTracker{}
}

// setError sets the first error encountered (thread-safe).
func (et *errorTracker) setError(err error) {
	et.mu.Lock()
	defer et.mu.Unlock()

	if et.error == nil {
		et.error = err
	}
}

// getError returns the first error encountered (if any).
func (et *errorTracker) getError() error {
	et.mu.Lock()
	defer et.mu.Unlock()
	return et.error
}

// loadBookmarkFromFile loads and validates a bookmark from a JSON file.
func loadBookmarkFromFile(path string) (*Bookmark, error) {
	var bj BookmarkJSON
	if err := files.JSONRead(path, &bj); err != nil {
		return nil, fmt.Errorf("JSON error in %s: %w", path, err)
	}
	if !validateChecksum(&bj) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidChecksum, path)
	}
	return NewFromJSON(&bj), nil
}

// loadConcurrently processes a single JSON file in a goroutine.
func loadConcurrently(
	path string,
	bs *slice.Slice[Bookmark],
	wg *sync.WaitGroup,
	sem chan struct{},
	errTracker *errorTracker,
) {
	sem <- struct{}{} // acquire semaphore
	wg.Add(1)

	go func(filePath string) {
		defer func() {
			<-sem     // release semaphore
			wg.Done() // mark goroutine as done
		}()

		b, err := loadBookmarkFromFile(filePath)
		if err != nil {
			errTracker.setError(err)
			return
		}

		bs.Push(b)
	}(path)
}

// LoadJSONBookmarks loads JSON bookmarks from a directory tree with
// controlled concurrency.
func LoadJSONBookmarks(repoPath string, bs *slice.Slice[Bookmark]) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	errTracker := newErrorTracker()

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			loadConcurrently(path, bs, &wg, sem, errTracker)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("filepath.Walk error: %w", err)
	}

	wg.Wait()

	return errTracker.getError()
}

// FindChanged returns the bookmarks that have changed since the last sync.
func FindChanged(oldBookmarks, newBookmarks []*Bookmark) []*Bookmark {
	oldChecksums := make(map[string]string) // URL -> checksum
	var changed []*Bookmark

	// build map of old checksums
	for i := range len(oldBookmarks) {
		oldChecksums[oldBookmarks[i].URL] = oldBookmarks[i].Checksum
	}
	// compare with new bookmarks
	for i := range len(newBookmarks) {
		if oldChecksum, exists := oldChecksums[newBookmarks[i].URL]; exists {
			if oldChecksum != newBookmarks[i].Checksum {
				changed = append(changed, newBookmarks[i])
			}
		} else {
			// new bookmark (didn't exist before)
			changed = append(changed, newBookmarks[i])
		}
	}

	return changed
}
