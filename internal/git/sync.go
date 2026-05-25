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

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
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
			}

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
func exportAsJSON(root string, bs []*bookmark.Bookmark, force bool) (bool, error) {
	var hasUpdates atomic.Bool
	g := new(errgroup.Group)

	for i := range bs {
		b := bs[i]
		g.Go(func() error {
			updated, err := bookio.SaveAsJSON(root, b, force)
			if err != nil {
				return err
			}

			if updated {
				hasUpdates.Store(true)
			}

			return nil
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
	g.SetLimit(runtime.NumCPU() * 2)

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

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
	g.SetLimit(runtime.NumCPU() * 2)

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

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

// Sync writes bookmarks to the repo and commits changes if any.
func Sync(ctx context.Context, app *application.App, mesg string) error {
	slog.DebugContext(ctx, "starting git sync")
	if !app.Git.Enabled {
		slog.DebugContext(ctx, "git sync disabled, skipping", "enabled", app.Git.Enabled)
		return nil
	}

	slog.DebugContext(ctx, "creating git repo instance")
	m, err := NewManager(app.Path.Database)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create git repo", "error", err)
		return err
	}

	if !m.IsTracked() {
		slog.DebugContext(ctx, "database path not tracked in git, skipping sync")
		return nil
	}

	r, err := db.New(ctx, app.Path.Database)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open database", "error", err)
		return err
	}
	defer r.Close()

	bs, err := r.All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch bookmarks", "error", err)
		return err
	}

	updated, err := m.Write(bs, app.Flags.Force)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write bookmarks to repo", "error", err)
		return err
	}

	if !updated {
		slog.DebugContext(ctx, "no changes detected, skipping commit")
		return nil
	}

	if err := m.Commit(mesg); err != nil {
		slog.ErrorContext(ctx, "failed to commit changes", "error", err)
		return err
	}

	slog.InfoContext(ctx, "git sync completed successfully", "changes_committed", updated)
	return nil
}
