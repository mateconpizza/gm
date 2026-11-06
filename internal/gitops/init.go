package gitops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/git"
)

type StorageOpt struct {
	Desc string
	Type string
}

var StorageOpts = []StorageOpt{
	{Type: "JSON", Desc: "Readable files. Anyone with access to the remote can read your bookmarks"},
	{Type: "YAML", Desc: "Readable files, easier to edit by hand"},
}

var EncryptionOpts = []StorageOpt{
	{Type: string(enc.AESGCM), Desc: "Secured with a passphrase"},
	{Type: string(enc.GPG), Desc: "Encrypt with GPG. Uses your existing keys"},
}

// Init initializes Git support and configures repository encryption.
func Init(ctx context.Context, app *application.App, c *ui.Console) error {
	p := c.Palette()

	gm := app.Git.Manager()

	if app.Flags.Reinit && gm.IsEnabled() {
		if err := confirmReinit(ctx, gm, c); err != nil {
			return err
		}
	}

	if err := gm.Init(ctx, app.Flags.Reinit); err != nil {
		if errors.Is(err, git.ErrGitInitialized) {
			return fmt.Errorf("%w, use %s", err, p.BrightYellow.Wrap("git init --reinit", p.Italic))
		}
		return err
	}

	if err := gm.CommitEmpty(ctx, CoreCommitMsg("initialized")); err != nil {
		return err
	}

	initHeader(c)
	if err := configureRepoOptions(ctx, app, c, gm); err != nil {
		return err
	}

	if err := c.Print(ctx, c.SuccessMesg("git initialized\n")); err != nil {
		return err
	}

	app.Git.Enabled = true
	return app.WriteConfig(true)
}

func initHeader(c *ui.Console) {
	p, f := c.Palette(), c.Frame()

	c.NewBannerBuilder().
		WithTitle("Storage options").
		WithSubtitle("Choose how bookmark files are stored in the Git repository.").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln()

	for _, s := range StorageOpts {
		var c ansi.Style
		switch s.Type {
		case "JSON":
			c = p.BrightGreen
		case "GPG":
			c = p.BrightBlue
		case "YAML":
			c = p.Orange
		}
		f.Midln(txt.PaddedLineWithWidth(c.Wrap(s.Type, p.Bold), s.Desc, 5))
	}

	f.Rowln().Flush()
}

func configureRepoOptions(ctx context.Context, app *application.App, c *ui.Console, gm *git.Mgr) error {
	def := "JSON"
	opts := []string{"YAML", def}
	if !app.Flags.Force && !app.Flags.Yes {
		opts = append(opts, "Abort")
		def = "Abort"
	}

	c.Term().SetChoicesFmt(terminal.ChoicesKeepCaseFmt)
	format, err := c.Choose(ctx, "How should bookmarks be stored?", opts, def)
	if err != nil {
		return err
	}

	if format == "Abort" {
		return app.Abort()
	}

	cfg := RepoConfig{Version: RepoConfigVersion, Format: RepoFormat(strings.ToLower(format)), Encryption: enc.None}
	if err := askForEncryption(ctx, app, gm, &cfg, c); err != nil {
		return err
	}

	return saveRepoConfig(ctx, gm, c, cfg)
}

func saveRepoConfig(ctx context.Context, gm *git.Mgr, c *ui.Console, cfg RepoConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := cfg.Save(gm.Root()); err != nil {
		return err
	}

	msg := CoreCommitMsg(fmt.Sprintf("%s storage", cfg.Format))
	if err := gm.Commit(ctx, msg); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("repo stores %s (encryption: %s)\n", cfg.Format, cfg.Encryption)))
}

// confirmReinit shows what `--reinit` is a destructive action.
func confirmReinit(ctx context.Context, gm *git.Mgr, c *ui.Console) error {
	p, f := c.Palette(), c.Frame()
	warn := p.BrightYellow
	pad := func(s, v any) string { return txt.PaddedLineWithWidth(s, v, 18) }

	f.Warning(warn.Sprint("warning: ")).
		Text(warn.Wrap("--reinit ", p.Italic)).
		Text(warn.Sprint("deletes the local git repository\n")).
		Midln(p.Dim.Sprint("this action cannot be undone")).
		Rowln().
		Info(pad("path:", gm.Root())).Ln().
		Info(pad("tracked:", fmt.Sprintf("%d database(s)", len(gm.Repos())))).Ln()

	unpushed, err := gm.Git().HasUnpushedCommits(ctx)
	switch {
	case err != nil:
		// usually no upstream configured, so there is nothing to compare with
		f.Info(pad("unpushed commits:", "unknown (no upstream?)\n"))
	case unpushed:
		f.Info(p.BrightRed.Sprint("unpushed commits will be lost\n"))
	}

	f.Rowln().Flush()

	return c.ConfirmErr(ctx, "delete and reinitialize?", "n")
}
