package gitops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

var ErrGitNoInformation = errors.New("no information")

func CoreCommitMsg(msg string) git.CommitMessage { return git.CommitMessage("[core] " + msg) }

// Info returns a prettify info of the repository.
func Info(ctx context.Context, d *deps.Deps) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	f, p := d.Console().Frame(), d.Console().Palette()

	gm, err := NewManager(app)
	if err != nil {
		return "", err
	}

	r, err := d.Repository()
	if err != nil {
		return "", err
	}
	defer r.Close()

	if !gm.IsTracked(app.DBBaseName()) || !gm.IsEnabled() {
		return f.StringReset(), err
	}

	f.Reset().
		HeaderCln(p.Red.With(p.Bold).Sprint, p.Red.Wrap("git:", p.Italic))

	gr := gm.NewRepo(r.BaseName())
	sum, err := gr.Summary()
	if err != nil {
		return f.StringReset(), err
	}

	// repo type
	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if gpg.IsInitialized(app.Path.Git()) {
		t = p.BrightMagenta.Wrap("GPG", p.Bold)
	}
	f.Rowln(txt.PaddedLine("type:", t))

	// remote
	if sum.GitRemote != "" {
		f.Rowln(txt.PaddedLine("remote:", sum.GitRemote))
	}

	// last git push
	if sum.LastSync != "" {
		tt, err := time.Parse(time.RFC3339, sum.LastSync)
		if err != nil {
			return f.StringReset(), err
		}

		lastSync := txt.RelativeTime(tt.Format(txt.TimeLayout)) +
			p.Gray.With(p.Italic).Sprintf(" (%s)", sum.LastSync)

		f.Rowln(txt.PaddedLine("last sync:", lastSync))
	}

	// unpushed commits
	g := gm.Git()
	unpushed, err := g.UnpushedCommits(ctx)
	if err != nil && !errors.Is(err, git.ErrGitNoUpstream) {
		return "", err
	}
	if unpushed > 0 {
		f.Rowln(txt.PaddedLine("unpushed:", unpushed))
	}

	// logging status
	f.Rowln(txt.PaddedLine("logging:", app.Git.Logging()))

	// enable status
	if app.GitEnabled() {
		f.Success(txt.PaddedLine("sync:", p.BrightGreen.Sprint("true"))).Ln()
	} else {
		f.Error(txt.PaddedLine("sync:", p.BrightRed.Sprint("false"))).Ln()
	}

	return f.StringReset(), nil
}

func InfoCmd(ctx context.Context, d *deps.Deps) error {
	i, err := Info(ctx, d)
	if err != nil {
		return err
	}

	if i == "" {
		return ErrGitNoInformation
	}

	c := d.Console()
	c.NewBannerBuilder().
		WithTitle("Git Information").
		WithTitleGlyph(d.Console().Glyphs().Square).
		WithSubtitle("showing current git status").
		Build().
		Rowln().
		Text(i).
		Flush()

	return nil
}

func ReadmePrompt(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	c := d.Console()
	if err := enableReadme(ctx, c, app); err != nil {
		return err
	}

	for _, path := range app.Databases() {
		r, err := db.New(ctx, path)
		if err != nil {
			return err
		}

		if err := app.SetDatabase(r.Name()); err != nil {
			return err
		}

		gm, err := NewManager(app)
		if err != nil {
			return err
		}

		if !gm.IsEnabled() || !gm.IsTracked(app.DBBaseName()) {
			continue
		}

		hook := HookReadme(app, BookmarksFromStore(r), StatsFromStore(r))

		if err := hook.Run(ctx, gm.Git()); err != nil {
			return err
		}
	}

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	return gm.Commit(ctx, CoreCommitMsg("add readme"))
}

func enableReadme(ctx context.Context, c *ui.Console, app *application.App) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r := app.Git.Readme
	p := c.Palette()
	readme := p.Bold.Sprint("README.md")

	if app.Flags.Configure {
		return configureReadme(ctx, app, c)
	}

	action, color := "Enable", p.BrightGreen
	if r.Enabled {
		action, color = "Disable", p.BrightYellow
	}
	if !c.Confirm(ctx, fmt.Sprintf("%s %s on git repositories?", color.Sprint(action), readme), "y") {
		return app.Abort()
	}
	r.Enabled = !r.Enabled

	if err := app.WriteConfig(true); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("settings saved\n"))
}

func configureReadme(ctx context.Context, app *application.App, c *ui.Console) error {
	glyphs, err := c.Choose(ctx, "Which glyphs to use?", bookio.SupportedGlyphs, "emoji")
	if err != nil {
		return err
	}

	summary, err := c.Choose(ctx, "Which summary style to use?", bookio.SupportedSummary, "badges")
	if err != nil {
		return err
	}

	r := app.Git.Readme
	r.Glyphs = glyphs
	r.Summary = summary

	if err := app.WriteConfig(true); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("settings saved\n"))
}
