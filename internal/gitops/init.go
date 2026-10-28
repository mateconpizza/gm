package gitops

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/git"
)

type Storage string

const (
	JSON Storage = "JSON"
	GPG  Storage = "GPG"
	YAML Storage = "YAML"
)

type StorageOption struct {
	Desc string
	Type Storage
}

var StorageOpts = []StorageOption{
	{Type: JSON, Desc: "Plaintext storage, easily viewable and inspectable"},
	{Type: GPG, Desc: "Encrypted storage, secure sync (experimental)"},
	{Type: YAML, Desc: "Plaintext storage, easily viewable and inspectable"},
}

// Init initializes Git support and configures repository encryption.
func Init(ctx context.Context, app *application.App, gm *git.Mgr) error {
	c := ui.NewDefaultConsole(app, app.Exit)
	p := c.Palette()

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
		case JSON:
			c = p.BrightGreen
		case GPG:
			c = p.BrightBlue
		case YAML:
			c = p.Orange
		}
		f.Midln(txt.PaddedLineWithWidth(c.Wrap(string(s.Type), p.Bold), s.Desc, 5))
	}

	f.Rowln().Flush()
}

func configureRepoOptions(ctx context.Context, app *application.App, c *ui.Console, gm *git.Mgr) error {
	opts := make([]string, 0, len(StorageOpts))
	for _, s := range StorageOpts {
		opts = append(opts, string(s.Type))
	}

	def := "JSON"
	if !app.Flags.Force && !app.Flags.Yes {
		opts = append(opts, "Abort")
		def = "Abort"
	}

	opt, err := c.Choose(ctx, "How should bookmarks be stored?", opts, def)
	if err != nil {
		return err
	}

	switch opt {
	case "GPG":
		if err := askForEncryption(ctx, app, c, gm); err != nil {
			return err
		}

	case "YAML":
		return application.ErrNotImplementedYet

	case "Abort":
		return app.Abort()
	default:
	}

	return nil
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
