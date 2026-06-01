package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
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
	if !k.IsTrusted() {
		return fmt.Errorf("%w: %s", gpg.ErrKeyNotTrusted, k.UserID)
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
