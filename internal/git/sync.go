package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

// Import clones a Git repository, parses its bookmark files, and imports them
// into the application.
func Import(a *app.Context, gm *Manager) ([]string, error) {
	urlRepo := a.Cfg.Flags.Path
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

	a.Console.Frame.Midln(fmt.Sprintf("Found %d repositorie/s", n)).Flush()

	var imported []string
	for _, repoName := range repos {
		dbPath, err := parseGitRepo(a, gm.RepoPath, repoName)
		if err != nil {
			if errors.Is(err, sys.ErrActionAborted) {
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
		return nil, sys.ErrActionAborted
	}

	if err := files.RemoveAll(gm.RepoPath); err != nil {
		return nil, fmt.Errorf("removing temp repo: %w", err)
	}

	fmt.Print(a.Console.SuccessMesg("imported bookmarks from git\n"))

	return imported, nil
}

// exportAsGPG export and encrypts the bookmarks and stores them in the git
// repo.
//
//nolint:funlen //ignore
func exportAsGPG(ctx context.Context, fingerprintPath, root string, bs []*bookmark.Bookmark) (bool, error) {
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

	g, ctx := errgroup.WithContext(ctx)
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

			if err := gpg.Encrypt(ctx, fingerprintPath, filePath, data); err != nil {
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
func exportAsJSON(root string, bs []*bookmark.Bookmark) (bool, error) {
	var (
		cfg        = config.New()
		hasUpdates uint32
	)

	g := new(errgroup.Group)

	for i := range bs {
		b := bs[i] // capture loop variable
		g.Go(func() error {
			updated, err := storeBookmarkAsJSON(root, b, cfg.Flags.Force)
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

// cleanGPGRepo removes the files from the git repo concurrently.
func cleanGPGRepo(ctx context.Context, root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git GPG files")

	g, ctx := errgroup.WithContext(ctx)
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
func cleanJSONRepo(ctx context.Context, root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git JSON files")

	g, ctx := errgroup.WithContext(ctx)
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
