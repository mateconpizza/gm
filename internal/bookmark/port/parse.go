package port

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

const FileExtJSON = ".json"

type loaderFileFn = func(path string) (*bookmark.Bookmark, error)

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

// deduplicate removes duplicate bookmarks.
func deduplicate(
	f *frame.Frame,
	r *db.SQLiteRepository,
	bs *slice.Slice[bookmark.Bookmark],
) ([]*bookmark.Bookmark, error) {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
		_, exists := r.Has(b.URL)
		return !exists
	})
	if originalLen != bs.Len() {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Warning(s + "\n").Flush()
	}

	if bs.Empty() {
		return nil, slice.ErrSliceEmpty
	}

	return bs.ItemsPtr(), nil
}

// deduplicate removes duplicate bookmarks.
func deduplicatePtr(f *frame.Frame, r *db.SQLiteRepository, bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	originalLen := len(bs)
	filtered := make([]*bookmark.Bookmark, 0, len(bs))

	for _, b := range bs {
		if _, exists := r.Has(b.URL); exists {
			continue
		}

		filtered = append(filtered, b)
	}

	n := len(filtered)
	if originalLen != n {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-n)
		f.Warning(s + "\n").Flush()
	}

	return filtered
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(
	t *terminal.Term,
	r *db.SQLiteRepository,
	bs *slice.Slice[bookmark.Bookmark],
) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Rowln()
	if _, err := deduplicate(f, r, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Midln("no new bookmark found, skipping import").Flush()
			return nil
		}
	}

	msg := fmt.Sprintf("scrape missing data from %d bookmarks found?", bs.Len())
	f.Rowln().Flush().Clear()
	if !config.App.Force {
		if err := t.ConfirmErr(f.Question(msg).String(), "y"); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}
	}

	if err := bookmark.ScrapeMissingDescription(bs); err != nil {
		return fmt.Errorf("scrapping missing description: %w", err)
	}
	return nil
}

func parseJSONRepo(f *frame.Frame, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Loading JSON bookmarks").String()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic, rotato.ColorStyleBold),
	)

	loader := func(path string) (*bookmark.Bookmark, error) {
		bj := &bookmark.BookmarkJSON{}
		if err := files.JSONRead(path, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !bookmark.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", bookmark.ErrInvalidChecksum, path)
		}

		count++
		sp.UpdateMesg(fmt.Sprintf("[%d] %s", count, filepath.Base(path)))

		b := bookmark.NewFromJSON(bj)

		mu.Lock()
		bookmarks = append(bookmarks, b)
		mu.Unlock()

		return b, nil
	}

	sp.Start()

	err := filepath.WalkDir(root, parseJSONFile(&wg, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	wg.Wait()
	err = errTracker.GetError()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	sp.UpdatePrefix(fmt.Sprintf("Loaded %d bookmarks", count))
	sp.Done()

	return bookmarks, nil
}

func parseGPGRepo(f *frame.Frame, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Decrypting bookmarks").String()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic, rotato.ColorStyleBold),
	)

	loader := func(path string) (*bookmark.Bookmark, error) {
		content, err := gpg.Decrypt(path)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		bj := &bookmark.BookmarkJSON{}
		if err := json.Unmarshal(content, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !bookmark.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", bookmark.ErrInvalidChecksum, path)
		}

		count++
		sp.UpdateMesg(fmt.Sprintf("[%d] %s", count, filepath.Base(path)))

		b := bookmark.NewFromJSON(bj)
		bookmarks = append(bookmarks, b)

		return b, nil
	}

	sp.Start()

	err := filepath.WalkDir(root, parseGPGFile(&wg, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	wg.Wait()
	err = errTracker.GetError()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	sp.UpdatePrefix(fmt.Sprintf("Decrypted %d bookmarks", count))
	sp.Done()

	return bookmarks, nil
}

// parseGPGFile is a WalkDirFunc that loads .gpg files concurrently.
func parseGPGFile(wg *sync.WaitGroup, mu *sync.Mutex, loader loaderFileFn) fs.WalkDirFunc {
	var (
		bs                 = slice.New[bookmark.Bookmark]()
		count              = 0
		errTracker         = NewErrorTracker()
		passphrasePrompted = false
		sem                = make(chan struct{}, runtime.NumCPU()*2)
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != gpg.Extension {
			return nil
		}
		// encrypt|decrypt the first item found, this will prompt the user
		// for the passphrase.
		if !passphrasePrompted {
			_, err = loader(path)
			if err != nil {
				return err
			}
			passphrasePrompted = true
			count--
			return nil
		}

		loadConcurrently(path, bs, wg, mu, sem, loader, errTracker)

		return nil
	}
}

func parseJSONFile(wg *sync.WaitGroup, mu *sync.Mutex, loader loaderFileFn) fs.WalkDirFunc {
	var (
		bs         = slice.New[bookmark.Bookmark]()
		errTracker = NewErrorTracker()
		sem        = make(chan struct{}, runtime.NumCPU()*2)
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != FileExtJSON {
			return nil
		}

		loadConcurrently(path, bs, wg, mu, sem, loader, errTracker)
		time.Sleep(5 * time.Millisecond)

		return nil
	}
}

// parseGitRepository loads a git repo into a database.
func parseGitRepository(root, repoName string, t *terminal.Term, f *frame.Frame) error {
	f.Clear().Rowln().Info(fmt.Sprintf(color.Text("Repository %q\n").Bold().String(), repoName))
	repoPath := filepath.Join(root, repoName)

	// read summary.json
	sum := git.NewSummary()
	if err := files.JSONRead(filepath.Join(repoPath, git.SummaryFileName), sum); err != nil {
		return fmt.Errorf("reading summary: %w", err)
	}

	f.Midln(format.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(format.PaddedLine("tags:", sum.RepoStats.Tags)).
		Midln(format.PaddedLine("favorites:", sum.RepoStats.Favorites)).Flush()

	if err := t.ConfirmErr(f.Rowln().Question("Import records from this repo?").String(), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	var (
		dbName = sum.RepoStats.Name
		dbPath = filepath.Join(config.App.Path.Data, dbName)
		opt    string
		err    error
	)
	if !files.Exists(dbPath) {
		opt = "new"
	} else {
		f.Clear().Warning(fmt.Sprintf("Database %q already exists\n", dbName)).Flush()
		f.Question("What do you want to do?")

		opt, err = t.Choose(f.String(), []string{"merge", "drop", "create", "select", "ignore"}, "m")
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		f.Clear()
	}

	return parseGitRepositoryAction(t, f, opt, dbPath, dbName, repoPath)
}

func parseGitRepositoryAction(t *terminal.Term, f *frame.Frame, opt, dbPath, dbName, repoPath string) error {
	switch strings.ToLower(opt) {
	case "new":
		if err := intoDB(f, dbPath, dbName, repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}
	case "c", "create":
		var dbName string
		for dbName == "" {
			dbName = files.EnsureSuffix(t.Prompt(f.Clear().Info("Enter new name: ").String()), ".db")
		}
		if err := intoDB(f, dbPath, dbName, repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}
	case "d", "drop":
		f.Warning("Dropping database\n").Flush()
		if err := db.DropFromPath(dbPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := mergeRecords(f.Clear(), dbPath, repoPath); err != nil {
			return err
		}
	case "m", "merge":
		f.Info("Merging database\n").Flush()
		if err := mergeRecords(f.Clear(), dbPath, repoPath); err != nil {
			return err
		}
	case "s", "select":
		if err := selectRecords(f.Clear(), dbPath, repoPath); err != nil {
			return err
		}
	case "i", "ignore":
		f.Clear().Warning("Skipping repository..." + dbName + "\n").Flush()
		return nil
	}

	return nil
}

// resolveFileConflictError resolves a file conflict error.
func resolveFileConflictError(rootPath string, err error, filePathJSON string, b *bookmark.Bookmark) error {
	if !errors.Is(err, files.ErrFileExists) {
		return err
	}
	bj := bookmark.BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return fmt.Errorf("%w", err)
	}
	// no need to update
	if bj.Checksum == b.Checksum {
		return nil
	}
	return gitStoreAsJSON(rootPath, b, true)
}

func gitUpdateJSON(root string, oldB, newB *bookmark.Bookmark) error {
	if err := GitCleanJSON(root, []*bookmark.Bookmark{oldB}); err != nil {
		return fmt.Errorf("%w", err)
	}

	return GitStore(newB)
}

// GitCleanJSON removes the files from the git repo.
func GitCleanJSON(root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git JSON files")
	for _, b := range bs {
		jsonPath, err := b.JSONPath()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		fname := filepath.Join(root, jsonPath)
		if err := files.RemoveFilepath(fname); err != nil {
			return fmt.Errorf("cleaning JSON: %w", err)
		}
	}

	return nil
}

// GitCleanGPG removes the files from the git repo.
func GitCleanGPG(root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git JSON files")
	for _, b := range bs {
		gpgPath, err := b.GPGPath()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fname := filepath.Join(root, gpgPath)
		if err := files.RemoveFilepath(fname); err != nil {
			if errors.Is(err, files.ErrFileNotFound) {
				return nil
			}
			return fmt.Errorf("cleaning GPG: %w", err)
		}
	}

	return nil
}
