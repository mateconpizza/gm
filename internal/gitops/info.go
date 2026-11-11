package gitops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/git"
)

var ErrGitNoInformation = errors.New("no information")

func CoreCommitMsg(msg string) git.CommitMessage {
	return git.CommitMessage(fmt.Sprintf("[%s] %s", git.RepoCoreName, msg))
}

// Info returns a prettify info of the repository.
func Info(ctx context.Context, d *deps.Deps) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	f, p := d.Console().Frame(), d.Console().Palette()

	r, err := d.Repository()
	if err != nil {
		return "", err
	}
	defer r.Close()

	gm := app.Git.Manager()
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
	rc, err := resolveConfig(app.Path.Git())
	if err != nil {
		return "", err
	}

	var t string
	switch rc.Format {
	case RepoFormatYAML:
		t = p.Orange.Wrap("YAML", p.Bold)
	default:
		t = p.BrightCyan.Wrap("JSON", p.Bold)
	}

	encryption := p.Italic.With(p.Gray).Sprintf(" (encryption: %s)", rc.Encryption)
	f.Rowln(txt.PaddedLine("type:", t+encryption))

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
