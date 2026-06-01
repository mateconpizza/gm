package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

func Track(ctx context.Context, r *db.SQLite, m *git.Mgr, gr *git.Repo) error {
	if m.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", git.ErrGitTracked, gr.Name())
	}

	bs, err := r.All(ctx)
	if err != nil {
		return err
	}

	stats := git.NewRepoStats()
	if err := r.Stats(ctx, stats); err != nil {
		return err
	}

	sum, err := gr.Summary()
	if err != nil {
		return err
	}

	stats.Name = gr.Name()
	sum.RepoStats = stats

	if err := gr.Add(ctx, bs); err != nil {
		return err
	}

	if err := m.Track(gr.Name()); err != nil {
		return err
	}

	if err := m.WriteRepos(); err != nil {
		return err
	}

	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	return m.Commit(ctx, fmt.Sprintf("[%s] add tracking", gr.Name()))
}

func Untrack(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(app.Path.Git(), git.WithGit(g))
	if err != nil {
		return err
	}

	gr := m.NewRepo(app.DBNameBase())
	commitMsg := fmt.Sprintf("[%s] remove tracking", gr.Name())
	if err := m.Untrack(ctx, gr, commitMsg); err != nil {
		return err
	}

	c := d.Console()
	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.Name())))
}

func TrackStatus(c *ui.Console, m *git.Mgr, gr *git.Repo) string {
	p := c.Palette()
	name := gr.Name()

	var sb strings.Builder
	if !m.IsTracked(name) {
		sb.WriteString(txt.PaddedLine(name, p.Gray.Wrap("(not tracked)\n", p.Italic)))
		return c.Error(sb.String()).StringReset()
	}

	var repoType string
	repoType = p.BrightMagenta.Wrap("JSON ", p.Bold)
	if gpg.IsInitialized(m.Root()) {
		repoType = p.BrightMagenta.Wrap("GPG ", p.Bold)
	}

	if name == files.StripSuffixes(application.MainDBName) {
		name = "main"
	}

	s := strings.TrimSpace(fmt.Sprintf("(%s)", gr.String()))
	sb.WriteString(txt.PaddedLine(name, repoType+p.Gray.Wrap(s, p.Italic)))

	return c.Success(sb.String() + "\n").String()
}
