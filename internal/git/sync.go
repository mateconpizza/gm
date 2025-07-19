package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	record "github.com/mateconpizza/gm/pkg/bookmark"
)

type loadFileFn func(path string) (*record.Bookmark, error)

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
		dbPath, err := parseGitRepo(c, gm.RepoPath, repoName)
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

// exportAsGPG export and encrypts the bookmarks and stores them in the git
// repo.
//
//nolint:funlen //ignore
func exportAsGPG(root string, bs []*record.Bookmark) (bool, error) {
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
	sp.Start()

	var count uint32
	n := len(bs)

	g, ctx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	for i := range bs {
		b := bs[i]

		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			hashPath, err := b.HashPath()
			if err != nil {
				return fmt.Errorf("hashing path: %w", err)
			}

			filePath := filepath.Join(root, hashPath+gpg.Extension)
			if files.Exists(filePath) {
				return nil // skip existing
			}

			if err := files.MkdirAll(filepath.Dir(filePath)); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}

			data, err := json.MarshalIndent(b.JSON(), "", "  ")
			if err != nil {
				return fmt.Errorf("json marshal: %w", err)
			}

			if err := gpg.Encrypt(filePath, data); err != nil {
				return fmt.Errorf("%w", err)
			}

			cur := atomic.AddUint32(&count, 1)
			sp.UpdatePrefix(f.Reset().Mid(fmt.Sprintf("Encrypting [%d/%d]", cur, n)).String())
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		sp.Fail("failed")
		return false, err
	}

	total := atomic.LoadUint32(&count)
	if total > 0 {
		sp.UpdatePrefix(f.Reset().Success(fmt.Sprintf("Encrypted [%d/%d]", total, n)).String())
		sp.Done("done")
	} else {
		sp.Done()
	}

	return total > 0, nil
}

// exportAsJSON creates the repository structure.
func exportAsJSON(root string, bs []*record.Bookmark) (bool, error) {
	var hasUpdates uint32

	g := new(errgroup.Group)

	for i := range bs {
		b := bs[i] // capture loop variable
		g.Go(func() error {
			updated, err := storeBookmarkAsJSON(root, b, config.App.Flags.Force)
			if err != nil {
				return err
			}
			if updated {
				atomic.StoreUint32(&hasUpdates, 1)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return false, err
	}

	return atomic.LoadUint32(&hasUpdates) == 1, nil
}

// extractFromGitRepo extracts records from a git repository.
func extractFromGitRepo(c *ui.Console, repoPath string) ([]*record.Bookmark, error) {
	if !files.Exists(repoPath) {
		return nil, fmt.Errorf("%w: %q", ErrGitRepoNotFound, repoPath)
	}

	rootDir := filepath.Dir(repoPath)
	return Read(c, rootDir)
}

// readJSONRepo extracts records from a JSON repository.
func readJSONRepo(c *ui.Console, root string, sp *rotato.Rotato) ([]*record.Bookmark, error) {
	var (
		count     uint32
		mu        sync.Mutex
		bookmarks = []*record.Bookmark{}
	)

	loader := func(path string) (*record.Bookmark, error) {
		bj := &record.BookmarkJSON{}
		if err := files.JSONRead(path, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !parser.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", record.ErrInvalidChecksum, path)
		}

		currentCount := atomic.AddUint32(&count, 1)
		sp.UpdateMesg(fmt.Sprintf("[%d] %s", currentCount, filepath.Base(path)))

		b := record.NewFromJSON(bj)

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
func readGPGRepo(c *ui.Console, root string, sp *rotato.Rotato) ([]*record.Bookmark, error) {
	var (
		count      uint32
		totalFiles int
		mu         sync.Mutex
		bookmarks  = []*record.Bookmark{}
	)

	gpgFiles, err := files.ListRecursive(root, ".gpg")
	if err != nil {
		return nil, err
	}
	totalFiles = len(gpgFiles)

	loader := func(path string) (*record.Bookmark, error) {
		content, err := gpg.Decrypt(path)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		bj := &record.BookmarkJSON{}
		if err := json.Unmarshal(content, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !parser.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", record.ErrInvalidChecksum, path)
		}

		currentCount := atomic.AddUint32(&count, 1)
		sp.UpdateMesg(fmt.Sprintf("[%d/%d] %s", currentCount, totalFiles, filepath.Base(path)))

		b := record.NewFromJSON(bj)
		bookmarks = append(bookmarks, b)

		return b, nil
	}

	sp.Start()

	// Create errgroup with context
	g, ctx := errgroup.WithContext(context.Background())

	err = filepath.WalkDir(root, parseGPGFile(ctx, g, &mu, loader))
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
func parseGPGFile(ctx context.Context, g *errgroup.Group, mu *sync.Mutex, loader loadFileFn) fs.WalkDirFunc {
	var (
		bs                 []*record.Bookmark
		passphrasePrompted bool
		sem                = semaphore.NewWeighted(int64(runtime.NumCPU() * 2))
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() ||
			filepath.Ext(path) != gpg.Extension ||
			filepath.Base(path) == SummaryFileName {
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

// parseJSONFile is a WalkDirFunc that loads .json files concurrently using a
// semaphore.
func parseJSONFile(ctx context.Context, g *errgroup.Group, mu *sync.Mutex, l loadFileFn) fs.WalkDirFunc {
	bs := []*record.Bookmark{}
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() ||
			filepath.Ext(path) != JSONFileExt ||
			filepath.Base(path) == SummaryFileName {
			return nil
		}

		loadConcurrently(ctx, path, &bs, g, mu, sem, l)
		return nil
	}
}

// cleanGPGRepo removes the files from the git repo concurrently.
func cleanGPGRepo(root string, bs []*record.Bookmark) error {
	slog.Debug("cleaning up git GPG files")

	g, ctx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	for _, b := range bs {
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			gpgPath, err := b.GPGPath(gpg.Extension)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			fname := filepath.Join(root, gpgPath)
			if err := files.Remove(fname); err != nil {
				if errors.Is(err, files.ErrFileNotFound) {
					return nil
				}

				return fmt.Errorf("cleaning GPG: %w", err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("cleaning GPG: %w", err)
	}

	return files.RemoveEmptyDirs(root)
}

// cleanJSONRepo removes the files from the git repo concurrently.
func cleanJSONRepo(root string, bs []*record.Bookmark) error {
	slog.Debug("cleaning up git JSON files")

	g, ctx := errgroup.WithContext(context.Background())
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	for _, b := range bs {
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			jsonPath, err := b.JSONPath()
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			fname := filepath.Join(root, jsonPath)
			if err := files.Remove(fname); err != nil {
				return fmt.Errorf("cleaning JSON: %w", err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("cleaning GPG: %w", err)
	}

	return files.RemoveEmptyDirs(root)
}

// loadConcurrently processes a single file in a goroutine using errgroup.
func loadConcurrently(
	ctx context.Context,
	path string,
	bs *[]*record.Bookmark,
	g *errgroup.Group,
	mu *sync.Mutex,
	sem *semaphore.Weighted,
	loader func(path string) (*record.Bookmark, error),
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
