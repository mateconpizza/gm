package gitops

import (
	"context"
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
	m, err := git.NewManager(app.Path.Git())
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

	f.Reset().Headerln(p.BrightRed.Wrap("git:", p.Italic))

	gr := m.NewRepo(r.BaseName())
	sum, err := m.Summary(gr)
	if err != nil {
		return f.StringReset(), err
	}

	// remote
	if sum.GitRemote != "" {
		f.Rowln(txt.PaddedLine("remote:", sum.GitRemote))
	}

	// repo type
	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if gpg.IsInitialized(app.Path.Home()) {
		t = p.BrightMagenta.Wrap("GPG", p.Bold)
	}
	f.Rowln(txt.PaddedLine("type:", t))

	if sum.LastSync != "" {
		tt, err := time.Parse(time.RFC3339, sum.LastSync)
		if err != nil {
			return f.StringReset(), err
		}

		lastSync := txt.RelativeTime(tt.Format(txt.TimeLayout)) +
			p.Gray.With(p.Italic).Sprintf(" (%s)", sum.LastSync)

		f.Rowln(txt.PaddedLine("last sync:", lastSync)).
			Success(txt.PaddedLine("sync:", true)).Ln()
	} else {
		f.Error(txt.PaddedLine("sync:", false)).Ln()
	}

	return f.StringReset(), nil
}
