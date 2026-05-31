//go:build ignore

package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

// exportAsGPG export and encrypts the bookmarks and stores them in the git
// repo.
func exportAsGPG(ctx context.Context, fingerprintPath, root string, bs []*bookmark.Bookmark) (bool, error) {
	if err := files.MkdirAll(root); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(ansi.Gray))
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Encrypting").String()),
		rotato.WithMessage("bookmarks..."),
		rotato.WithMessageColor(rotato.FgYellow),
		rotato.WithSpinnerColor(rotato.FgYellow),
		rotato.WithDoneMessageColor(rotato.FgBrightGreen, rotato.StyleItalic),
		rotato.WithFailMessageColor(rotato.FgBrightRed),
	)
	sp.Start()

	var count atomic.Uint32
	n := len(bs)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU() * 2)

	for i := range bs {
		b := bs[i]

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
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

				cur := count.Add(1)
				sp.UpdatePrefix(
					f.Reset().
						Mid(fmt.Sprintf("Encrypting [%d/%d]", cur, n)).
						String(),
				)
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		sp.Fail("failed")
		return false, err
	}

	total := count.Load()
	if total > 0 {
		sp.UpdatePrefix(f.Reset().Success(fmt.Sprintf("Encrypted [%d/%d]", total, n)).String())
		sp.Done("done")
	} else {
		sp.Done()
	}

	return total > 0, nil
}

// exportAsJSON creates the repository structure.
func exportAsJSON(ctx context.Context, root string, bs []*bookmark.Bookmark, force bool) (bool, error) {
	var hasUpdates atomic.Bool
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU() * 2)

	for i := range bs {
		b := bs[i]
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				updated, err := bookio.SaveAsJSON(root, b, force)
				if err != nil {
					return err
				}

				if updated {
					hasUpdates.Store(true)
				}

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return false, err
	}

	return hasUpdates.Load(), nil
}

// cleanGPGRepo removes the files from the git repo concurrently.
func cleanGPGRepo(ctx context.Context, root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git GPG files")

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(1)

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				gpgPath, err := b.GPGPath()
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
			}
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
	g.SetLimit(1)

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				jsonPath, err := b.JSONPath()
				if err != nil {
					return fmt.Errorf("%w", err)
				}

				fname := filepath.Join(root, jsonPath)
				if err := files.Remove(fname); err != nil {
					return fmt.Errorf("cleaning JSON: %w", err)
				}

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("cleaning JSON: %w", err)
	}

	return files.RemoveEmptyDirs(root)
}
