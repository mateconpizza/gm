package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"
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
)

type loadFileFn = func(path string) (*bookmark.Bookmark, error)

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
		sp.UpdatePrefix(f.Reset().Success(fmt.Sprintf("Encrypted [%d/%d]", count, n)).String())
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

// Import clones a Git repository, parses its bookmark files, and imports them
// into the application.
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

func intoDBFromGit(c *ui.Console, gr *Repository) error {
	bookmarks, err := extractFromGitRepo(c, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	r, err := db.Init(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	c.F.Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), gr.Loc.DBName)).Flush()

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
func mergeAndInsert(c *ui.Console, gr *Repository) error {
	r, err := db.New(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := extractFromGitRepo(c, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = port.Deduplicate(c, r, bookmarks)
	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gr.Export(); err != nil {
		return err
	}
	if err := gr.Commit("imported bookmarks from git"); err != nil {
		return err
	}

	n := len(bookmarks)
	if n > 0 {
		c.F.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, gr.Loc.DBName)).Flush()
	}

	return nil
}

// readJSONRepo extracts records from a JSON repository.
func readJSONRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count     = 0
		mu        sync.Mutex
		bookmarks = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Loading JSON bookmarks").String()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
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

	// Create errgroup with context
	g, ctx := errgroup.WithContext(context.Background())

	err := filepath.WalkDir(root, parseJSONFile(ctx, g, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	sp.UpdatePrefix(c.Success(fmt.Sprintf("Loaded %d bookmarks", count)).StringReset())
	sp.Done("done!")

	return bookmarks, nil
}

// readGPGRepo extracts records from a GPG repository.
func readGPGRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count     = 0
		mu        sync.Mutex
		bookmarks = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Decrypting bookmarks").StringReset()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
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

	// Create errgroup with context
	g, ctx := errgroup.WithContext(context.Background())

	err := filepath.WalkDir(root, parseGPGFile(ctx, g, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	sp.UpdatePrefix(c.Success(fmt.Sprintf("Decrypted %d bookmarks", count)).StringReset())
	sp.Done("done!")

	return bookmarks, nil
}

// parseGPGFile is a WalkDirFunc that loads .gpg files concurrently using a
// semaphore and prompts once for the GPG passphrase.
// parseGPGFile is a WalkDirFunc that loads .gpg files concurrently using a semaphore.
func parseGPGFile(ctx context.Context, g *errgroup.Group, mu *sync.Mutex, loader loadFileFn) fs.WalkDirFunc {
	// TODO:
	// - [ ] replace `passphrasePrompted` with `sync.Once`? maybe? read more.
	var (
		bs                 []*bookmark.Bookmark
		passphrasePrompted bool
		sem                = semaphore.NewWeighted(int64(runtime.NumCPU() * 2))
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != gpg.Extension || filepath.Base(path) == SummaryFileName {
			return nil
		}

		// Prompt for GPG passphrase on the first valid file
		if !passphrasePrompted {
			if _, err := loader(path); err != nil {
				return err
			}
			passphrasePrompted = true
			return nil
		}

		loadConcurrently(ctx, path, &bs, g, mu, sem, loader)
		return nil
	}
}

func parseJSONFile(
	ctx context.Context,
	g *errgroup.Group,
	mu *sync.Mutex,
	loader loadFileFn,
) fs.WalkDirFunc {
	bs := []*bookmark.Bookmark{}
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != JSONFileExt || filepath.Base(path) == SummaryFileName {
			return nil
		}

		loadConcurrently(ctx, path, &bs, g, mu, sem, loader)
		return nil
	}
}

// loadConcurrently processes a single file in a goroutine using errgroup.
func loadConcurrently(
	ctx context.Context,
	path string,
	bs *[]*bookmark.Bookmark,
	g *errgroup.Group,
	mu *sync.Mutex,
	sem *semaphore.Weighted,
	loader func(path string) (*bookmark.Bookmark, error),
) {
	// Use errgroup.Go instead of manual goroutine + WaitGroup management
	g.Go(func() error {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}
		defer sem.Release(1)

		b, err := loader(path)
		if err != nil {
			return err
		}

		mu.Lock()
		*bs = append(*bs, b)
		mu.Unlock()

		return nil
	})
}
