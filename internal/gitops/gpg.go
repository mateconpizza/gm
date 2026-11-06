package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/git"
)

const (
	// gcmGitAttribute defines the Git attributes rule for encrypted
	// files.
	gcmGitAttribute = "*.enc binary"

	// gpgGitAttribute defines the Git attributes rule for encrypted files.
	gpgGitAttribute = "*.gpg diff=gpg"
)

func askForGPGEncryption(ctx context.Context, app *application.App, c *ui.Console, gm *git.Mgr) error {
	fr := gpg.NewKeyResolver(app.Path.Git())
	if fr.Initialized() {
		return nil
	}

	bin, err := sys.Which(gpg.Command)
	if err != nil {
		slog.Debug("git repo with GPG, command not found", "command", gpg.Command)
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
	}

	fps, err := fr.List(ctx)
	if err != nil {
		return err
	}

	m := menuFingerprint(c, app)
	key, err := selectFingerprint(ctx, m, fps)
	if err != nil {
		return err
	}

	p, f := c.Palette(), c.Frame()
	fn := func(s string) string { return p.Italic.Wrap(s, p.Bold) }

	f.Success("GPG command found: " + fn(bin)).Ln().
		Rowln().
		Flush()

	c.NewBannerBuilder().
		WithTitle("GPG Keys").
		WithSubtitle("Using GPG key " + fn(key.UserID)).
		Render()

	return setupGPG(ctx, c, gm, key)
}

func askForEncryption(ctx context.Context, app *application.App, gm *git.Mgr, cfg *RepoConfig, c *ui.Console) error {
	c.Term().SetChoicesFmt(terminal.ChoicesInlineBracketedFmt)
	if !c.Confirm(ctx, "Use encryption?", "n") {
		return nil
	}
	encryptionHeader(c)

	c.Term().SetChoicesFmt(terminal.ChoicesKeepCaseFmt)
	defer c.Term().SetChoicesFmt(terminal.ChoicesInlineBracketedFmt)

	opt, err := c.Choose(ctx, "Which one?", []string{"GPG", "AES-GCM", "None"}, "None")
	if err != nil {
		return err
	}

	// setup first, so a failed setup leaves no config behind
	switch opt {
	case "GPG":
		cfg.Encryption = enc.GPG
		if err := askForGPGEncryption(ctx, app, c, gm); err != nil {
			return err
		}
	case "AES-GCM":
		cfg.Encryption = enc.AESGCM
		if err := initAESGCM(app.Path.Git(), git.AttributesFile); err != nil {
			return err
		}
	}

	return nil
}

func encryptionHeader(c *ui.Console) {
	p, f := c.Palette(), c.Frame()

	c.NewBannerBuilder().
		WithTitle("Encryption options").
		WithSubtitle("Choose how bookmark files are encrypted in the Git repository.").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln()

	for _, s := range EncryptionOpts {
		var c ansi.Style
		switch s.Type {
		case string(enc.GPG):
			c = p.BrightBlue
		case string(enc.AESGCM):
			c = p.Orange
		}
		f.Midln(txt.PaddedLineWithWidth(c.Wrap(strings.ToUpper(s.Type), p.Bold), s.Desc, 5))
	}

	f.Rowln().Flush()
}

// promptGPGPassphrase handles unlocking and initializing the first GPG file.
func promptGPGPassphrase(ctx context.Context, f *bookio.FileLoader, sp spinner, path string, prompted *bool) error {
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

	sp.UpdateMesg("waiting for GPG passphrase")
	sp.SetMessageDecorator(func(msg string) string {
		remaining := max(time.Until(deadline).Round(time.Second), 0)
		// *prompted will be true for any subsequent spinner updates after this function returns
		if remaining == 0 || *prompted {
			return msg
		}
		return msg + " " + fmt.Sprintf("(%.0fs left)", remaining.Seconds())
	})

	// blocks until the user types their passphrase in the GPG prompt
	if _, err := f.Loader(ctxPrompt, path); err != nil {
		return err
	}

	return nil
}

func menuFingerprint(c *ui.Console, app *application.App) *menu.Menu[*gpg.Fingerprint] {
	// FIX: if the is invalid, Trusted: UNKNOWN, make it all dimmer.
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
		menu.WithHeader("select a fingerprint"),
		menu.WithKeybinds(menu.KeymapTogglePreview()),
		menu.WithHeaderKeymaps(),
		menu.WithInterruptFn(func(err error) { app.Exit(err) }),
		menu.WithMultilineView(),
		menu.WithPreviewCmd(gpg.Command+" --list-keys {+4}"),
	)

	m.SetFormatter(func(f *gpg.Fingerprint) string {
		return fmt.Sprintf(
			"[Trusted: %s] %s: %s %s: %s\n%s: %s",
			trustColor(f),
			p.BrightBlue.Wrap("KeyID", p.Bold),
			f.KeyID,
			p.BrightMagenta.Wrap("UserID", p.Bold),
			f.UserID,
			p.BrightYellow.Wrap("Fingerprint", p.Bold),
			f.Fingerprint,
		)
	})

	return m
}

func selectFingerprint(ctx context.Context, m *menu.Menu[*gpg.Fingerprint], fps []*gpg.Fingerprint) (*gpg.Fingerprint, error) {
	keys, err := m.Select(ctx, fps)
	if err != nil {
		return nil, err
	}

	key := keys[0]

	return key, nil
}

func setupGPG(ctx context.Context, c *ui.Console, gm *git.Mgr, k *gpg.Fingerprint) error {
	if err := k.Validate(); err != nil {
		return fmt.Errorf("gpg init: %w", err)
	}

	if err := which(gpg.Command); err != nil {
		return fmt.Errorf("%w: %s", err, gpg.Command)
	}

	if err := initGPG(gm.Root(), git.AttributesFile, k); err != nil {
		return fmt.Errorf("gpg init: %w", err)
	}

	// add diff to git config
	for k, v := range gpg.GitDiffConf {
		if err := gm.SetCfg(ctx, k, strings.Join(v, " ")); err != nil {
			return err
		}
	}

	if err := gm.Commit(ctx, CoreCommitMsg("gpg initialized")); err != nil {
		return err
	}

	fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("GPG repo initialized with key %q", k.UserID)))

	return nil
}

// initGPG will extract the gpg fingerprint and save it to .gpg-id.
func initGPG(path, gitAttrFile string, fingerprint *gpg.Fingerprint) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("%w", err)
	}

	fileIDPath := filepath.Join(path, gpg.FingerprintIDFilename)
	err := os.WriteFile(fileIDPath, []byte(fingerprint.Fingerprint+"\n"), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write .gpg-id: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, gitAttrFile), []byte(gpgGitAttribute), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	return nil
}

func which(c string) error {
	_, err := exec.LookPath(c)
	if err != nil {
		return exec.ErrNotFound
	}
	return nil
}
