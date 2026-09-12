package gitops

import (
	"context"
	"errors"
	"time"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/git"
)

// Info returns a prettify info of the repository.
func Info(ctx context.Context, d *deps.Deps) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	f, p := d.Console().Frame(), d.Console().Palette()

	gm, err := NewManager(&ManagerConfig{
		Root:    app.Path.Git(),
		Writer:  app.Git.Writer(),
		Version: app.Version(),
	})
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

	f.Reset().Textln(p.BrightRed.Wrap("git:", p.Italic))

	gr := gm.NewRepo(r.BaseName())
	sum, err := gr.Summary()
	if err != nil {
		return f.StringReset(), err
	}

	// repo type
	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if gpg.IsInitialized(app.Path.Home()) {
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
		f.Success(txt.PaddedLine("sync:", p.BrightGreen.Wrap("true", p.Bold))).Ln()
	} else {
		f.Error(txt.PaddedLine("sync:", p.BrightRed.Wrap("false", p.Bold))).Ln()
	}

	return f.StringReset(), nil
}

func InfoCmd(ctx context.Context, d *deps.Deps) error {
	i, err := Info(ctx, d)
	if err != nil {
		return err
	}

	d.Console().NewBannerBuilder().
		WithTitle("Git Information").
		WithTitleColor(d.Console().Palette().BrightYellow).
		WithSubtitle("showing current git status").
		Build().
		Rowln().
		HeaderC(d.Console().Palette().BrightRed, i).
		Flush()

	return nil
}
