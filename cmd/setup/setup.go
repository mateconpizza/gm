// Package setup provides commands for initializing and configuring the bookmark database.
// It handles database creation, initial schema setup, and optional Git repository tracking.
package setup

import (
	"context"
	"fmt"
	"strings"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type Inserter interface {
	InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error)
}

const padding = 28

var InitCmd = &cobra.Command{
	Use:               "init",
	Short:             "initialize a new bookmarks database",
	Hidden:            true,
	Annotations:       cli.SkipDBCheck,
	PersistentPreRunE: cli.HookCheckIfDatabaseInitialized,
	PostRunE:          InitAppPostFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: convert `InitCmd` to command builder(*application.App)
		app, err := application.FromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		return initializeAction(
			cmd.Context(),
			deps.New(
				deps.WithApplication(app),
				deps.WithConsole(ui.NewDefaultConsole(app, app.Exit)),
			),
		)
	},
}

func NewCmd(_ *application.App) *cobra.Command {
	return InitCmd
}

func initializeAction(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	c, p := d.Console(), d.Console().Palette()

	// announce app version
	header := func() string { return p.BrightYellow.Wrap(c.Glyphs().Square+" ", p.Bold) }
	c.Frame().
		CustomFunc(header, app.PrettyVersion()).
		Rowln().
		Flush()

	if err := initWorkspace(c, app); err != nil {
		return err
	}

	r, err := db.Init(ctx, app.Path.DB())
	if err != nil {
		return fmt.Errorf("database init failed: %w", err)
	}
	d.WithRepo(r)

	if err := dbops.MigrationsStatus(ctx, d); err != nil {
		return err
	}
	defer r.Close()

	if app.DBName != application.MainDBName {
		c.Frame().Success("Initialized database: " + c.Palette().Italic.Sprint(app.DBName) + "\n").Flush()
		return nil
	}

	return seedNewRepo(ctx, app, r, c)
}

// InitAppPostFunc ask user to track new database if git is initialized.
func InitAppPostFunc(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	app, err := application.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !app.GitEnabled() {
		return nil
	}

	gm, err := gitops.NewManager(app)
	if err != nil {
		return err
	}

	name := app.DBBaseName()
	if gm.IsTracked(name) {
		return nil
	}

	c := ui.NewDefaultConsole(app, app.Exit)

	if !c.Confirm(ctx, fmt.Sprintf("Track database %q?", name), "n") {
		c.ReplaceLine(c.Warning(fmt.Sprintf("Skipping database %q", name)).String())
		return nil
	}
	c.ReplaceLine(c.Success(fmt.Sprintf("Tracking database %q", name)).String())

	if err := files.MkdirAll(app.Path.DB()); err != nil {
		return fmt.Errorf("creating repo path: %w", err)
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}

	gr := gm.NewRepo(r.Name(),
		gitops.RepoFileReader(app.Flags.Color),
		gitops.RepoFileRemover(),
		gitops.RepoFileWriter(app.Flags.Color),
	)

	if err := gitops.Track(ctx, r, app, gm, gr); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("database %q tracked\n", name)))
}

// initWorkspace creates the paths for the application.
func initWorkspace(c *ui.Console, app *application.App) error {
	if files.Exists(app.Path.Home()) {
		return nil
	}

	p, f := c.Palette(), c.Frame()
	dimmer := func(s string) string { return p.Dim.Wrap(s, p.Italic) }
	header := func() string { return p.BrightYellow.Wrap(c.Glyphs().Square+" ", p.Bold) }
	f.CustomFunc(header, p.Bold.Sprint("Initializing workspace")).Ln().
		Mid(txt.PaddedLineWithWidth("path", dimmer(app.Path.Home()), padding)).Ln().
		Mid(txt.PaddedLineWithWidth("database", dimmer(app.DBName), padding)).Ln().
		Rowln().
		Flush()

	if err := app.CreatePaths(); err != nil {
		return fmt.Errorf("failed to create workspace paths: %w", err)
	}

	return nil
}

func seedNewRepo(ctx context.Context, app *application.App, r Inserter, c *ui.Console) error {
	ib := bookmark.New()
	ib.URL = app.Info.URL
	ib.Title = app.Info.Title
	ib.Tags = bookmark.ParseTags(app.Info.Tags)
	ib.Desc = app.Info.Short

	if _, err := r.InsertOne(ctx, ib); err != nil {
		return fmt.Errorf("failed to seed initial bookmark: %w", err)
	}

	p, f := c.Palette(), c.Frame()
	header := func() string { return p.BrightYellow.Wrap(c.Glyphs().Square+" ", p.Bold) }
	f.CustomFunc(header, p.Bold.Sprint("Seeding initial bookmark")).Ln()

	u := strings.Replace(ib.URL, "https://", "", 1)

	f.Success(txt.PaddedLineWithWidth("inserted", p.Dim.Wrap(u, p.Italic), padding)).Ln().
		Rowln().
		Success("Setup complete\n").
		Flush()

	return nil
}
