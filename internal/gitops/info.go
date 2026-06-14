package gitops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
	m, err := NewManager(app)
	if err != nil {
		return "", err
	}

	r, err := d.Repository()
	if err != nil {
		return "", err
	}
	defer r.Close()

	if !m.IsTracked(app.DBBaseName()) || !m.IsEnabled() {
		return f.StringReset(), err
	}

	f.Reset().
		HeaderCln(p.BrightRed, p.BrightRed.Wrap("git:", p.Italic))

	gr := m.NewRepo(r.BaseName())
	sum, err := m.Summary(gr)
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
	g := m.Git()
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
	c := d.Console()
	p := c.Palette()

	title := p.BrightYellow.With(p.Bold).
		Sprint("Git Information")
	subtitle := p.Dim.With(p.Italic).
		Sprint("showing current git status")
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	d.Console().Frame().SetBorders(frame.WithBordersSmallBlock2())

	c.Frame().
		CustomFunc(header, title).Ln().
		Headerln(subtitle).
		Rowln().
		Flush()

	i, err := Info(ctx, d)
	if err != nil {
		return err
	}

	fmt.Fprint(d.Writer(), i)

	return nil
}
