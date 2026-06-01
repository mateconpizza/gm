package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

func gpgStrategy(fingerprintPath string) *bookio.RepositoryLoader {
	return &bookio.RepositoryLoader{
		Func:   gpgBookmarkFileLoader(fingerprintPath),
		Prefix: "GPG bookmarks [%d/%d]",
		FileFilter: bookio.And(
			bookio.IsFile,
			bookio.HasExtension(gpg.Extension),
			bookio.NotNamed(git.SummaryFileName),
		),
	}
}

// ReadGPGRepo handles reading encrypted GPG bookmark repositories.
func ReadGPGRepo(ctx context.Context, cfg RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(cfg.loader.Func)

	cfg.sp.Start()
	defer cfg.sp.Done()

	var passphrasePrompted bool

	if err := filepath.WalkDir(cfg.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, cfg.root, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !cfg.loader.FileFilter(path, d) {
			return nil
		}

		// Handle prompt for GPG passphrase on the first valid file
		if !passphrasePrompted {
			if err := promptGPGPassphrase(ctx, f, cfg.sp, path, &passphrasePrompted); err != nil {
				return err
			}
			passphrasePrompted = true
		}

		f.LoadAsync(ctx, path)

		cfg.sp.UpdatePrefix(fmt.Sprintf(cfg.loader.Prefix, f.Count(1), cfg.total))
		cfg.sp.UpdateMesg("decrypting..." + filepath.Base(path))

		return nil
	}); err != nil {
		cfg.sp.Fail(err.Error())
		return nil, err
	}

	return f.Results()
}

// gpgBookmarkFileLoader returns a loader function that decrypts and parses
// GPG-encrypted bookmark.
func gpgBookmarkFileLoader(fingerprintPath string) func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	return func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		content, err := gpg.Decrypt(ctx, fingerprintPath, path)
		if err != nil {
			return nil, fmt.Errorf("decrypting %w", err)
		}

		bj := &bookmark.BookmarkJSON{}
		if err := json.Unmarshal(content, bj); err != nil {
			fmt.Println(string(content))
			fmt.Println(path)
			return nil, fmt.Errorf("error unmarshalling JSON: %w", err)
		}

		return bookmark.NewFromJSON(bj), nil
	}
}

// promptGPGPassphrase handles unlocking and initializing the first GPG file.
func promptGPGPassphrase(
	ctx context.Context,
	f *bookio.FileLoader,
	sp *rotato.Rotato,
	path string,
	prompted *bool,
) error {
	unlocked, err := gpg.Unlocked(ctx, path)
	if err != nil {
		return err
	}

	if unlocked {
		if _, err := f.Loader(ctx, path); err != nil {
			return err
		}
		return nil
	}

	ctxPrompt, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	deadline, _ := ctxPrompt.Deadline()
	dimmer := rotato.FgYellow.With(rotato.StyleDim, rotato.StyleBold)

	sp.UpdateMesg("waiting for GPG passphrase")
	sp.SetMessageDecorator(func(mesg string) string {
		remaining := max(time.Until(deadline).Round(time.Second), 0)
		// *prompted will be true for any subsequent spinner updates after this function returns
		if remaining == 0 || *prompted {
			return mesg
		}
		return mesg + " " + dimmer.Sprintf("(%.0fs left)", remaining.Seconds())
	})

	// blocks until the user types their passphrase in the GPG prompt
	if _, err := f.Loader(ctxPrompt, path); err != nil {
		return err
	}

	return nil
}
