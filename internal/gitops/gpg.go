package gitops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

func AskForEncryption(ctx context.Context, c *ui.Console, app *application.App, m *git.Mgr) error {
	if gpg.IsInitialized(app.Path.Git()) {
		return nil
	}

	_, err := sys.Which(gpg.Command)
	if err != nil {
		slog.Debug("git repo with GPG, command not found", "command", gpg.Command)
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
	}

	p := c.Palette()
	c.Frame().Rowln().Success("GPG command found").Ln().Flush()
	if !c.Confirm(ctx, "Use GPG for encryption? "+p.BrightRed.Wrap("(experimental)", p.Italic), "n") {
		return nil
	}

	fps, err := gpg.ListFingerprints()
	if err != nil {
		return err
	}

	mf := menuFingerprint(c, app)
	key, err := selectFingerprint(mf, fps)
	if err != nil {
		return err
	}

	return initGPG(ctx, c, m, key)
}

func gpgStrategy(recipient string) (*bookio.RepositoryLoader, error) {
	g, err := gpg.New(recipient)
	if err != nil {
		return nil, err
	}

	return &bookio.RepositoryLoader{
		Func:   gpgBookmarkFileLoader(g),
		Prefix: "GPG bookmarks [%d/%d]",
		FileFilter: bookio.And(
			bookio.IsFile,
			bookio.HasExtension(gpg.Extension),
			bookio.NotNamed(git.SummaryFileName),
		),
	}, nil
}

func addGPGFiles(ctx context.Context, bs []*bookmark.Bookmark, sp *rotato.Rotato, repoPath string) error {
	root := filepath.Dir(repoPath)
	fingerprintPath := gpg.GPGIDPath(root)

	fp, err := gpg.LookupKey(fingerprintPath)
	if err != nil {
		return fmt.Errorf("gpg strategy: %w", err)
	}

	if err := fp.Validate(); err != nil {
		return err
	}

	g, err := gpg.New(fp.Fingerprint)
	if err != nil {
		return err
	}

	var (
		current atomic.Uint32
		total   = len(bs)
	)
	for i := range bs {
		sp.UpdateMesg(fmt.Sprintf("[%d/%d] encrypting bookmarks files", current.Add(1), total))
		if err := createGPGFile(ctx, g, repoPath, bs[i]); err != nil {
			return err
		}
	}

	return nil
}

func createGPGFile(ctx context.Context, g *gpg.GPG, repoPath string, b *bookmark.Bookmark) error {
	fullpath, err := genFullpath(repoPath, b)
	if err != nil {
		return fmt.Errorf("gpgfile: %w", err)
	}

	if files.Exists(fullpath) {
		slog.Warn("gpgfile: not found", "file", fullpath)
		return nil
	}

	if err := files.MkdirAll(filepath.Dir(fullpath)); err != nil {
		return fmt.Errorf("gpgfile: failed creating dir: %w, %q", err, filepath.Dir(fullpath))
	}

	data, err := json.MarshalIndent(b.JSON(), "", "  ")
	if err != nil {
		return fmt.Errorf("gpgfile: JSON marshal: %w", err)
	}

	if err := g.Encrypt(ctx, fullpath, data); err != nil {
		return fmt.Errorf("gpgfile: creating file: %w", err)
	}

	return nil
}

// ReadGPGRepo handles reading encrypted GPG bookmark repositories.
func ReadGPGRepo(ctx context.Context, cfg RepoReaderCfg) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(cfg.loader.Func)

	cfg.sp.Start(ctx)
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
func gpgBookmarkFileLoader(g *gpg.GPG) bookio.LoaderFileFunc {
	return func(ctx context.Context, path string) (*bookmark.Bookmark, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		content, err := g.Decrypt(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("decrypting %w", err)
		}

		bj := &bookmark.BookmarkJSON{}
		if err := json.Unmarshal(content, bj); err != nil {
			return nil, fmt.Errorf("error unmarshalling JSON: %w, %s", err, path)
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

func menuFingerprint(c *ui.Console, app *application.App) *menu.Menu[*gpg.Fingerprint] {
	p := c.Palette()
	trustColor := func(key *gpg.Fingerprint) string {
		t := key.TrustLevelString()
		if key.IsTrusted() {
			return p.BrightGreen.Sprint(strings.ToUpper(t))
		}

		switch t {
		case "marginal":
			return p.BrightYellow.Sprint(strings.ToUpper(t))
		default:
			return p.BrightRed.Sprint(strings.ToUpper(t))
		}
	}

	m := picker.New[*gpg.Fingerprint](
		app,
		menu.WithArgs("--no-bold"),
		menu.WithHeader(" select a fingerprint "),
		menu.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
		menu.WithMultilineView(),
		menu.WithPreview(gpg.Command+" --list-keys {+4}"),
	)

	m.SetFormatter(func(f **gpg.Fingerprint) string {
		fp := *f
		return fmt.Sprintf(
			"[Trusted: %s] %s: %s %s: %s\n%s: %s",
			trustColor(fp),
			p.BrightBlue.Wrap("KeyID", p.Bold),
			fp.KeyID,
			p.BrightMagenta.Wrap("UserID", p.Bold),
			fp.UserID,
			p.BrightYellow.Wrap("Fingerprint", p.Bold),
			fp.Fingerprint,
		)
	})

	return m
}

func selectFingerprint(m *menu.Menu[*gpg.Fingerprint], fps []*gpg.Fingerprint) (*gpg.Fingerprint, error) {
	keys, err := m.Select(fps)
	if err != nil {
		return nil, err
	}

	key := keys[0]

	return key, nil
}

func initGPG(ctx context.Context, c *ui.Console, m *git.Mgr, k *gpg.Fingerprint) error {
	if err := k.Validate(); err != nil {
		return fmt.Errorf("gpg init: %w", err)
	}

	if err := gpg.Init(m.Root(), git.AttributesFile, k); err != nil {
		return fmt.Errorf("gpg init: %w", err)
	}

	// add diff to git config
	for k, v := range gpg.GitDiffConf {
		if err := m.SetCfg(ctx, k, strings.Join(v, " ")); err != nil {
			return err
		}
	}

	if err := m.Commit(ctx, "[core] gpg repo initialized"); err != nil {
		return err
	}

	fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("GPG repo initialized with key %q", k.UserID)))

	return nil
}
