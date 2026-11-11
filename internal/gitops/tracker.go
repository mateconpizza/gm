package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/enc"
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

	sess := app.Session().
		WithConfirmation(true).
		WithValidation(true)

	gr := NewRepo(gm, r.BaseName(), sess, app.Flags.Color)
	RegisterHooks(r, app, gr)

	c := d.Console()
	if gm.IsTracked(gr.Name()) {
		fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", gr.Name())))
	}

	if err := Track(ctx, r, app, gm, gr); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Name())))
}

func Track(ctx context.Context, r store, app *application.App, gm *git.Mgr, gr *git.Repo) error {
	if gm.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", git.ErrGitTracked, gr.Name())
	}

	bs, err := r.All(ctx)
	if err != nil {
		return err
	}

	if err := gr.Add(ctx, bs); err != nil {
		return err
	}

	if err := gm.Track(gr.Name()); err != nil {
		return err
	}

	if err := gm.WriteRepos(); err != nil {
		return err
	}

	return gm.Commit(ctx, gr.CommitMsg(git.Add, "tracking"))
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

	gr := gm.NewRepo(app.DBBaseName())
	if err := gm.Untrack(ctx, gr); err != nil {
		return err
	}

	c := d.Console()
	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.Name())))
}

func TrackStatus(c *ui.Console, gm *git.Mgr, gr *git.Repo, f RepoFormat) string {
	p := c.Palette()
	name := gr.Name()

	var sb strings.Builder
	if !gm.IsTracked(name) {
		sb.WriteString(txt.PaddedLine(name, p.Gray.Wrap("(not tracked)\n", p.Italic)))
		return c.Error(sb.String()).StringReset()
	}

	var repoType string
	switch f {
	case RepoFormatYAML:
		repoType = p.Orange.Sprint(RepoFormatYAML + " ")
	case RepoFormatJSON:
		repoType = p.BrightMagenta.Sprint(RepoFormatJSON + " ")
	}

	if name == files.StripExts(application.MainDBName) {
		name = "main"
	}

	s := strings.TrimSpace(p.Gray.With(p.Italic).Sprintf("(%s)", gr.String()))
	sb.WriteString(txt.PaddedLine(name, repoType+s))

	return c.Success(sb.String() + "\n").String()
}

func TrackMgr(ctx context.Context, app *application.App, gm *git.Mgr, c *ui.Console, dbFiles []string) error {
	c.NewBannerBuilder().
		WithTitle("Git Tracker Databases").
		WithComment(" (ctrl-c to exit)").
		WithSubtitle("select which databases to track").
		Build().
		Rowln().
		Flush()

	dbFiles, trackAll, err := trackSelection(ctx, dbFiles, c.Choose, app)
	if err != nil {
		return err
	}

	files.PrioritizeFile(dbFiles, application.MainDBName)

	sess := app.Session().
		WithConfirmation(true)

	for _, dbPath := range dbFiles {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return application.ErrExitFailure
			}

			return err
		}

		name := files.StripExts(filepath.Base(dbPath))
		if gm.IsTracked(name) {
			fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", name)))
			continue
		}

		if !trackAll && !c.Confirm(ctx, fmt.Sprintf("Track %q?", name), "n") {
			continue
		}

		r, err := db.New(ctx, dbPath)
		if err != nil {
			return err
		}

		gr := NewRepo(gm, r.BaseName(), sess, app.Flags.Color)
		RegisterHooks(r, app, gr)

		if err := Track(ctx, r, app, gm, gr); err != nil {
			return err
		}

		r.Close()

		c.ReplaceLine(c.Success(fmt.Sprintf("%q tracked", name)).String())
	}

	return nil
}

func TrackMgrStatus(app *application.App, c *ui.Console) error {
	gm, err := NewManager(app)
	if err != nil {
		return err
	}

	if len(gm.Repos()) == 0 {
		slog.Debug("tracker: no repos found")
		return app.Failure()
	}

	dbFiles, err := files.Find(app.Path.Home(), "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.NewBannerBuilder().
		WithTitle("Git Tracked Databases").
		WithSubtitle("showing tracked databases with git").
		Build().
		Rowln()

	rc, err := resolveConfig(gm.Root())
	if err != nil {
		return err
	}

	if rc.Encryption != enc.None {
		c.Frame().
			Midln(txt.PaddedLine("encryption:", c.Palette().BrightBlue.Sprint(rc.Encryption))).
			Rowln()
	}

	c.Frame().Flush()

	var tracked strings.Builder
	var untracked strings.Builder

	// move main database to the top
	files.PrioritizeFile(dbFiles, app.DBName)

	for _, dbPath := range dbFiles {
		name := filepath.Base(dbPath)
		gr := gm.NewRepo(name)

		s := TrackStatus(c, gm, gr, rc.Format)
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

type chooseFunc func(ctx context.Context, q string, opts []string, def string) (string, error)

func trackSelection(ctx context.Context, dbFiles []string, choose chooseFunc, app *application.App) (result []string, trackAll bool, err error) {
	if len(dbFiles) <= 1 {
		return dbFiles, false, nil
	}

	opt, err := choose(ctx, "track all?", []string{"yes", "select", "no"}, "y")
	if err != nil {
		return nil, false, err
	}

	result = dbFiles

	switch opt {
	case "select":
		h := "select database/s to track " +
			app.Glyphs().Sep +
			" use <TAB> for multi-selection"
		result, err = dbops.NewDatabaseSelector(app).
			WithOpts(
				menu.WithHeader(h),
				menu.WithMultiSelection(),
				menu.WithHeaderKeymaps(),
			).
			Select(ctx)
		if err != nil {
			return nil, false, err
		}
		trackAll = true

	case "yes":
		trackAll = true
	}

	return result, trackAll, nil
}
