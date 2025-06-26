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
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

const JSONFileExt = ".json"

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

// Deduplicate removes duplicate bookmarks.
func Deduplicate(
	c *ui.Console,
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
		c.Warning(s + "\n").Flush()
	}

	return bs.ItemsPtr(), nil
}

// deduplicate removes duplicate bookmarks.
func deduplicatePtr(c *ui.Console, r *db.SQLiteRepository, bs []*bookmark.Bookmark) []*bookmark.Bookmark {
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
		c.Warning(s + "\n").Flush()
	}

	return filtered
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(
	c *ui.Console,
	r *db.SQLiteRepository,
	bs []*bookmark.Bookmark,
) ([]*bookmark.Bookmark, error) {
	bs = deduplicatePtr(c, r, bs)
	if len(bs) == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return bs, nil
	}

	if !config.App.Flags.Force {
		if err := c.ConfirmErr(fmt.Sprintf("scrape missing data from %d bookmarks found?", len(bs)), "y"); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				return bs, nil
			}

			return nil, fmt.Errorf("%w", err)
		}
	}

	if err := bookmark.ScrapeMissingDescription(bs); err != nil {
		return nil, fmt.Errorf("scrapping missing description: %w", err)
	}

	return bs, nil
}

// parseJSONRepo extracts records from a JSON repository.
func parseJSONRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Loading JSON bookmarks").String()),
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

func parseGPGRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Decrypting bookmarks").StringReset()),
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

		if d.IsDir() || filepath.Ext(path) != JSONFileExt {
			return nil
		}

		loadConcurrently(path, bs, wg, mu, sem, loader, errTracker)
		time.Sleep(1 * time.Millisecond)

		return nil
	}
}

// parseGitRepository loads a git repo into a database.
func parseGitRepository(c *ui.Console, root, repoName string) (string, error) {
	c.F.Rowln().Info(fmt.Sprintf(color.Text("Repository %q\n").Bold().String(), repoName))
	repoPath := filepath.Join(root, repoName)

	// read summary.json
	sum := git.NewSummary()
	if err := files.JSONRead(filepath.Join(repoPath, git.SummaryFileName), sum); err != nil {
		return "", fmt.Errorf("reading summary: %w", err)
	}

	c.F.Midln(txt.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(txt.PaddedLine("tags:", sum.RepoStats.Tags)).
		Midln(txt.PaddedLine("favorites:", sum.RepoStats.Favorites)).Flush()

	if err := c.ConfirmErr("Import records from this repo?", "y"); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	var (
		dbName = sum.RepoStats.Name
		dbPath = filepath.Join(config.App.Path.Data, dbName)
		opt    string
		err    error
	)

	if files.Exists(dbPath) {
		c.Warning(fmt.Sprintf("Database %q already exists\n", dbName)).Flush()

		opt, err = c.Choose(
			"What do you want to do?",
			[]string{"merge", "drop", "create", "select", "ignore"},
			"m",
		)
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
	} else {
		opt = "new"
	}

	resultPath, err := parseGitRepositoryOpt(c, opt, dbPath, repoPath)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

// parseGitRepositoryOpt handles the options for parseGitRepository.
func parseGitRepositoryOpt(c *ui.Console, o, dbPath, repoPath string) (string, error) {
	switch strings.ToLower(o) {
	case "new":
		if err := intoDBFromGit(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "c", "create":
		var dbName string
		for dbName == "" {
			dbName = files.EnsureSuffix(c.Prompt("Enter new name: "), ".db")
		}

		dbPath = filepath.Join(filepath.Dir(dbPath), dbName)
		if err := intoDBFromGit(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "d", "drop":
		c.Warning("Dropping database\n").Flush()

		if err := db.DropFromPath(dbPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}

		if err := mergeRecords(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "m", "merge":
		c.Info("Merging database\n").Flush()

		if err := mergeRecords(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "s", "select":
		if err := selectRecords(c, dbPath, repoPath); err != nil {
			if errors.Is(err, menu.ErrFzfActionAborted) {
				return "", nil
			}

			return "", err
		}

	case "i", "ignore":
		repoName := files.StripSuffixes(filepath.Base(dbPath))
		c.ReplaceLine(
			c.Warning(fmt.Sprintf("%s repo %q", color.Yellow("skipping"), repoName)).StringReset(),
		)

		return "", nil
	}

	return dbPath, nil
}

// resolveFileConflictErr resolves a file conflict error.
func resolveFileConflictErr(rootPath string, err error, filePathJSON string, b *bookmark.Bookmark) error {
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

	return storeBookmarkAsJSON(rootPath, b, true)
}

func gitUpdateJSON(root string, oldB, newB *bookmark.Bookmark) error {
	if err := cleanJSONRepo(root, []*bookmark.Bookmark{oldB}); err != nil {
		return fmt.Errorf("%w", err)
	}

	return GitStore(newB)
}

// cleanJSONRepo removes the files from the git repo.
func cleanJSONRepo(root string, bs []*bookmark.Bookmark) error {
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

// cleanGPGRepo removes the files from the git repo.
func cleanGPGRepo(root string, bs []*bookmark.Bookmark) error {
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
