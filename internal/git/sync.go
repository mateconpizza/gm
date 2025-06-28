package git

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

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

// exportAsGPG export and encrypts the bookmarks and stores them in the git
// repo.
func exportAsGPG(root string, bs []*bookmark.Bookmark) (bool, error) {
	if err := files.MkdirAll(root); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Encrypting").String()),
		rotato.WithMesg("bookmarks..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithFailColorMesg(rotato.ColorBrightRed),
	)

	n := len(bs)
	count := 0

	for i := range n {
		hashPath, err := bs[i].HashPath()
		if err != nil {
			return false, fmt.Errorf("hashing path: %w", err)
		}

		filePath := filepath.Join(root, hashPath+gpg.Extension)
		if files.Exists(filePath) {
			continue
		}

		dir := filepath.Dir(filePath)
		if err := files.MkdirAll(dir); err != nil {
			return false, fmt.Errorf("mkdir: %w", err)
		}

		data, err := json.MarshalIndent(bs[i].ToJSON(), "", "  ")
		if err != nil {
			return false, fmt.Errorf("json marshal: %w", err)
		}

		if err := gpg.Encrypt(filePath, data); err != nil {
			return false, fmt.Errorf("%w", err)
		}

		sp.Start()
		count++
		sp.UpdatePrefix(f.Reset().Mid(fmt.Sprintf("Encrypting [%d/%d]", count, n)).String())
	}

	if count > 0 {
		sp.Done("done")
	} else {
		sp.Done()
	}

	return count > 0, nil
}

// exportAsJSON creates the repository structure.
func exportAsJSON(root string, bs []*bookmark.Bookmark) (bool, error) {
	var hasUpdates bool
	for _, b := range bs {
		updated, err := storeBookmarkAsJSON(root, b, config.App.Flags.Force)
		if err != nil {
			return hasUpdates, err
		}

		if updated {
			hasUpdates = true
		}
	}

	return hasUpdates, nil
}

// Import imports bookmarks from a git repository.
func Import(c *ui.Console, gm *Manager, urlRepo string) ([]string, error) {
	if err := gm.Clone(urlRepo); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	repos, err := files.ListRootFolders(gm.RepoPath, ".git")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	n := len(repos)
	if n == 0 {
		return nil, ErrGitRepoNotFound
	}

	c.F.Midln(fmt.Sprintf("Found %d repositorie/s", n)).Flush()

	var imported []string
	for _, repoName := range repos {
		dbPath, err := parseGitRepository(c, gm.RepoPath, repoName)
		if err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				s := color.Yellow("skipping")
				c.ReplaceLine(c.Warning(fmt.Sprintf("%s repo %q", s, repoName)).StringReset())
				n--
				continue
			}

			return nil, fmt.Errorf("%w", err)
		}

		if dbPath != "" {
			imported = append(imported, dbPath)
		}
	}

	if len(imported) == 0 {
		return nil, terminal.ErrActionAborted
	}

	if err := files.RemoveAll(gm.RepoPath); err != nil {
		return nil, fmt.Errorf("removing temp repo: %w", err)
	}

	fmt.Print(c.SuccessMesg("imported bookmarks from git\n"))

	return imported, nil
}

func intoDBFromGit(c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	r, err := db.Init(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	c.F.Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), filepath.Base(dbPath))).Flush()

	return nil
}

// extractFromGitRepo extracts records from a git repository.
func extractFromGitRepo(c *ui.Console, repoPath string) ([]*bookmark.Bookmark, error) {
	if !files.Exists(repoPath) {
		return nil, fmt.Errorf("%w: %q", ErrGitRepoNotFound, repoPath)
	}

	rootDir := filepath.Dir(repoPath)
	if !gpg.IsInitialized(rootDir) {
		return readJSONRepo(c, repoPath)
	}

	return readGPGRepo(c, repoPath)
}

// mergeAndInsert merges non-duplicates records into database.
func mergeAndInsert(c *ui.Console, dbPath, repoPath string) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = port.Deduplicate(c, r, bookmarks)
	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(bookmarks)
	if n > 0 {
		c.F.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
	}

	return nil
}

// selectAndInsert prompts the user to select records to import.
func selectAndInsert(c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return err
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithArgs("--cycle"),
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
	)

	records := make([]bookmark.Bookmark, 0, len(bookmarks))
	for _, b := range bookmarks {
		records = append(records, *b)
	}

	slices.SortFunc(records, func(a, b bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	m.SetItems(records)
	m.SetPreprocessor(bookmark.Oneline)

	selected, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := make([]*bookmark.Bookmark, 0, len(selected))
	for i := range selected {
		bs = append(bs, &selected[i])
	}

	debookmarks := port.Deduplicate(c, r, bs)
	if err := r.InsertMany(context.Background(), debookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(debookmarks)
	if n > 0 {
		c.F.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
	}

	return nil
}

// loadConcurrently processes a single JSON file in a goroutine.
func loadConcurrently(
	ctx context.Context,
	path string,
	bs *[]*bookmark.Bookmark,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	sem *semaphore.Weighted,
	loader func(path string) (*bookmark.Bookmark, error),
	errTracker *ErrTracker,
) {
	if err := sem.Acquire(ctx, 1); err != nil {
		errTracker.SetError(err)
		return
	}
	wg.Add(1)

	go func(filePath string) {
		defer func() {
			sem.Release(1)
			wg.Done()
		}()

		b, err := loader(filePath)
		if err != nil {
			errTracker.SetError(err)
			return
		}

		mu.Lock()
		*bs = append(*bs, b)
		mu.Unlock()
	}(path)
}
