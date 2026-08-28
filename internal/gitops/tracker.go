package gitops

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

func NewTrack(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	gr := NewRepo(gm, r.Name(), git.WithRepoStore(r))

	c := d.Console()
	if gm.IsTracked(gr.Name()) {
		fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", gr.Name())))
	}

	if err := Track(ctx, r, gm, gr); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Name())))
}

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

	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	gr := NewRepo(gm, app.DBBaseName())
	commitMsg := fmt.Sprintf("[%s] remove tracking", gr.Name())
	if err := gm.Untrack(ctx, gr, commitMsg); err != nil {
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

	if name == files.StripExts(application.MainDBName) {
		name = "main"
	}

	s := strings.TrimSpace(fmt.Sprintf("(%s)", gr.String()))
	sb.WriteString(txt.PaddedLine(name, repoType+p.Gray.Wrap(s, p.Italic)))

	return c.Success(sb.String() + "\n").String()
}

func TrackMgr(ctx context.Context, gm *git.Mgr, c *ui.Console, dbFiles []string) error {
	p := c.Palette()
	title := p.BrightYellow.With(p.Bold).
		Sprint("Git Tracker Databases")
	subtitle := p.Dim.With(p.Italic).
		Sprint("select which databases to track")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	c.Frame().
		CustomFunc(header, title+comment).Ln().
		Headerln(subtitle).
		Rowln().
		Flush()

	files.PrioritizeFile(dbFiles, application.MainDBName)

	for i, dbPath := range dbFiles {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return sys.ErrExitFailure
			}

			return err
		}

		name := files.StripExts(filepath.Base(dbPath))
		if gm.IsTracked(name) {
			fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", name)))
			continue
		}

		if !c.Confirm(ctx, fmt.Sprintf("Track %q?", name), "n") {
			continue
		}

		r, err := db.New(ctx, dbPath)
		if err != nil {
			return err
		}

		gr := NewRepo(gm, r.Name(), git.WithRepoStore(r))
		if err := Track(ctx, r, gm, gr); err != nil {
			return err
		}

		r.Close()

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking %q", name)).String())
		if i != len(dbFiles)-1 {
			fmt.Fprintln(c.Writer())
		}
	}

	return nil
}

func TrackMgrStatus(c *ui.Console, app *application.App) error {
	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	if len(gm.Repos()) == 0 {
		return nil
	}

	p := c.Palette()

	title := p.BrightYellow.With(p.Bold).
		Sprint("Git Tracked Databases")
	subtitle := p.Dim.With(p.Italic).
		Sprint("showing tracked databases with git")
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	dbFiles, err := files.Find(app.Path.Home(), "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	// move main database to the top
	files.PrioritizeFile(dbFiles, app.DBName)

	c.Frame().
		CustomFunc(header, title).Ln().
		Headerln(subtitle).
		Rowln().
		Flush()

	var tracked strings.Builder
	var untracked strings.Builder

	for _, dbPath := range dbFiles {
		name := filepath.Base(dbPath)
		gr := NewRepo(gm, name)

		s := TrackStatus(c, gm, gr)
		if s == "" {
			continue
		}

		if gm.IsTracked(gr.Name()) {
			tracked.WriteString(s)
			continue
		}

		untracked.WriteString(s)
	}

	fmt.Fprint(c.Writer(), tracked.String())
	fmt.Fprint(c.Writer(), untracked.String())

	return nil
}
