package bookmark

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
)

var ErrInvalidChecksum = errors.New("invalid checksum")

// ExportBookmarks creates the repository structure.
func ExportBookmarks(root string, bs []*Bookmark) error {
	for _, b := range bs {
		if err := StoreAsJSON(root, b, config.App.Force); err != nil {
			return err
		}
	}

	return nil
}

func StoreAsGPG(root string, bookmarks []*Bookmark) error {
	root = filepath.Join(root, files.StripSuffixes(config.App.DBName))
	if err := files.MkdirAll(root); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp := rotato.New(
		rotato.WithPrefix("Encrypting"),
		rotato.WithMesg("bookmarks..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithFailColorMesg(rotato.ColorBrightRed),
	)

	n := len(bookmarks)
	count := 0
	for i := range n {
		hashPath, err := bookmarks[i].HashPath()
		if err != nil {
			return fmt.Errorf("hashing path: %w", err)
		}
		if err := gpg.Create(root, hashPath, bookmarks[i].ToJSON()); err != nil {
			if errors.Is(err, files.ErrFileExists) {
				continue
			}
			return fmt.Errorf("creating GPG file: %w", err)
		}
		sp.Start()
		count++
		sp.UpdatePrefix(fmt.Sprintf("Encrypting [%d/%d]", count, n))
	}
	if count > 0 {
		sp.Done("done")
	} else {
		sp.Done()
	}

	return nil
}

// StoreAsJSON creates files structure.
func StoreAsJSON(rootPath string, b *Bookmark, force bool) error {
	domain, err := Domain(b.URL)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// domainPath: root -> data -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return fmt.Errorf("%w", err)
	}
	// urlHash := domainPath -> urlHash.json
	urlHash := HashURL(b.URL)
	filePathJSON := filepath.Join(domainPath, urlHash+".json")
	if err := files.JSONWrite(filePathJSON, b.ToJSON(), force); err != nil {
		return resolveFileConflictError(rootPath, err, filePathJSON, b)
	}

	return nil
}

// resolveFileConflictError resolves a file conflict error.
func resolveFileConflictError(rootPath string, err error, filePathJSON string, b *Bookmark) error {
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
	return StoreAsJSON(rootPath, b, true)
}

// CleanupGitFiles removes the files associated with a bookmark.
func CleanupGitFiles(root string, b *Bookmark, extension string) error {
	slog.Debug("cleaning up git files")
	hashPath, err := b.HashPath()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fname := filepath.Join(root, hashPath+extension)
	if !files.Exists(fname) {
		slog.Debug("file not found", "path", fname)
		return nil
	}
	if err := files.Remove(fname); err != nil {
		return fmt.Errorf("removing file:%w", err)
	}
	// check if the directory is empty
	fdir := filepath.Dir(fname)
	dirs, err := files.List(fdir, "*")
	if err != nil {
		return fmt.Errorf("listing directory: %w", err)
	}
	if len(dirs) == 0 {
		// remove empty path
		if err := files.Remove(fdir); err != nil {
			return fmt.Errorf("removing directory: %w", err)
		}
	}
	return nil
}

func ValidateChecksumJSON(b *BookmarkJSON) bool {
	tags := ParseTags(strings.Join(b.Tags, ","))
	return b.Checksum == Checksum(b.URL, b.Title, b.Desc, tags)
}

func ValidateChecksum(b *Bookmark) bool {
	return b.Checksum == Checksum(b.URL, b.Title, b.Desc, b.Tags)
}

// ErrTracker provides thread-safe error tracking.
type ErrTracker struct {
	mu    sync.Mutex
	error error
}

// NewErrorTracker creates a new error tracker.
func NewErrorTracker() *ErrTracker {
	return &ErrTracker{}
}

// SetError sets the first error encountered (thread-safe).
func (et *ErrTracker) SetError(err error) {
	et.mu.Lock()
	defer et.mu.Unlock()

	if et.error == nil {
		et.error = err
	}
}

// GetError returns the first error encountered (if any).
func (et *ErrTracker) GetError() error {
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
	if !ValidateChecksumJSON(&bj) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidChecksum, path)
	}
	return NewFromJSON(&bj), nil
}

func LoadConcurrentlyFake(
	path string,
	bs *slice.Slice[Bookmark],
	wg *sync.WaitGroup,
	sem chan struct{},
	loader func(path string) (*Bookmark, error),
	errTracker *ErrTracker,
) {
}

// LoadConcurrently processes a single JSON file in a goroutine.
func LoadConcurrently(
	path string,
	bs *slice.Slice[Bookmark],
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	sem chan struct{},
	loader func(path string) (*Bookmark, error),
	errTracker *ErrTracker,
) {
	sem <- struct{}{} // acquire semaphore
	wg.Add(1)

	go func(filePath string) {
		defer func() {
			<-sem     // release semaphore
			wg.Done() // mark goroutine as done
		}()

		b, err := loader(filePath)
		if err != nil {
			errTracker.SetError(err)
			return
		}

		bs.Push(b)
	}(path)
}

// LoadBookmarksFromPath loads JSON bookmarks from a directory tree with
// controlled concurrency.
func LoadBookmarksFromPath(root string, bs *slice.Slice[Bookmark]) error {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	sem := make(chan struct{}, runtime.NumCPU()*2)
	errTracker := NewErrorTracker()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".json" {
			LoadConcurrently(path, bs, &wg, &mu, sem, loadBookmarkFromFile, errTracker)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("filepath.Walk error: %w", err)
	}

	wg.Wait()

	return errTracker.GetError()
}

// LoadJSONBookmarks loads JSON bookmarks from a directory tree with
// controlled concurrency.
func LoadJSONBookmarks(root string, bs *slice.Slice[Bookmark]) error {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	sem := make(chan struct{}, runtime.NumCPU()*2)
	errTracker := NewErrorTracker()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".json" {
			LoadConcurrently(path, bs, &wg, &mu, sem, loadBookmarkFromFile, errTracker)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("filepath.Walk error: %w", err)
	}

	wg.Wait()

	return errTracker.GetError()
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
