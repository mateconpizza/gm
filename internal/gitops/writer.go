//go:build ignore

package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
	gitnew "github.com/mateconpizza/gm/pkg/git"
)

func jsonWriter(ctx context.Context, dstDir string) gitnew.FileWriter {
	g := new(errgroup.Group)
	done := make(chan struct{})

	return &gitnew.Writer{
		Group:  g,
		DoneCh: done,
		Writer: func(b *bookmark.Bookmark) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if _, err := bookio.SaveAsJSON(dstDir, b, true); err != nil {
				return err
			}
			return nil
		},
	}
}

func gpgWriter(ctx context.Context, fingerprintPath, dstDir string, total int) gitnew.FileWriter {
	var (
		count atomic.Uint32
		g     = new(errgroup.Group)
		done  = make(chan struct{})
	)

	sp := rotato.New(
		rotato.WithPrefix("GPG bookmarks"),
		rotato.WithMessage("encrypting..."),
		rotato.WithMessageColor(rotato.StyleDim),
		rotato.WithPrefixColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
	)
	sp.Start()

	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		sp.Done()
	}()

	return &gitnew.Writer{
		Group:  g,
		DoneCh: done,
		Writer: func(b *bookmark.Bookmark) error {
			hashPath, err := b.HashPath()
			if err != nil {
				return fmt.Errorf("hashing path: %w", err)
			}

			path := filepath.Join(dstDir, hashPath+gpg.Extension)
			if files.Exists(path) {
				return nil // skip existing
			}

			if err := files.MkdirAll(filepath.Dir(path)); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}

			data, err := json.MarshalIndent(b.JSON(), "", "  ")
			if err != nil {
				return fmt.Errorf("json marshal: %w", err)
			}

			if err := gpg.Encrypt(ctx, fingerprintPath, path, data); err != nil {
				return fmt.Errorf("%w", err)
			}

			cur := count.Add(1)
			sp.UpdatePrefix(fmt.Sprintf("GPG bookmarks [%d/%d]", cur, total))
			return nil
		},
	}
}

// func NewRepoWriter(ctx context.Context, gitRoot, dstDir string, total int) gitnew.FileWriter {
// 	if gpg.IsInitialized(gitRoot) {
// 		fingerprintPath := gpg.GPGIDPath(gitRoot)
// 		return gpgWriter(ctx, fingerprintPath, dstDir, total)
// 	}
// 	return jsonWriter(ctx, dstDir)
// }
