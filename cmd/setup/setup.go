// Package setup provides commands for initializing and configuring the bookmark database.
// It handles database creation, initial schema setup, and optional Git repository tracking.
package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

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

		return initializeAction(deps.New(
			cmd.Context(),
			deps.WithApplication(app),
			deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
		))
	},
}

func NewCmd(_ *application.App) *cobra.Command {
	return InitCmd
}

func initializeAction(d *deps.Deps) error {
	app, err := d.Application()
	if err != nil {
		return err
	}

	c := d.Console()

	// announce app version
	c.Frame().Header(app.PrettyVersion()).Rowln().Flush()

	if err := initWorkspace(c, app); err != nil {
		return err
	}

	r, err := initMigrations(d.Context(), app, c)
	if err != nil {
		return err
	}
	defer r.Close()

	if app.DBName != application.MainDBName {
		c.Frame().Success("Initialized database: " + c.Palette().Italic.Sprint(app.DBName) + "\n").Flush()
		return nil
	}

	return seedNewRepo(d.Context(), app, r, c)
}

// InitAppPostFunc ask user to track new database if git is initialized.
func InitAppPostFunc(cmd *cobra.Command, _ []string) error {
	app, err := application.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !app.Git.Enabled {
		return nil
	}
	gr, err := git.NewRepo(app.Path.Database)
	if err != nil {
		return err
	}

	if gr.IsTracked() {
		return nil
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
	if !c.Confirm(fmt.Sprintf("Track database %q?", gr.Loc.DBName), "n") {
		c.ReplaceLine(c.Warning(fmt.Sprintf("Skipping database %q", gr.Loc.DBName)).String())
		return nil
	}
	c.ReplaceLine(c.Success(fmt.Sprintf("Tracking database %q", gr.Loc.DBName)).String())

	if err := files.MkdirAll(gr.Loc.Path); err != nil {
		return fmt.Errorf("creating repo path: %w", err)
	}

	if err := gr.Track(); err != nil {
		return err
	}

	fmt.Println(c.SuccessMesg(fmt.Sprintf("database %q tracked", gr.Loc.DBName)))

	return nil
}

// initWorkspace creates the paths for the application.
func initWorkspace(c *ui.Console, app *application.App) error {
	if files.Exists(app.Path.Data) {
		return nil
	}

	p, f := c.Palette(), c.Frame()
	dimmer := func(s string) string { return p.Dim.Wrap(s, p.Italic) }

	f.Headerln(p.Bold.Sprint("Initializing workspace")).
		Info(txt.PaddedLineWithPad("path", dimmer(app.Path.Data)+"\n", padding)).
		Info(txt.PaddedLineWithPad("database", dimmer(app.DBName)+"\n", padding)).
		Rowln().
		Flush()

	if err := app.CreatePaths(); err != nil {
		return fmt.Errorf("failed to create workspace paths: %w", err)
	}

	return nil
}

func seedNewRepo(ctx context.Context, app *application.App, r *db.SQLite, c *ui.Console) error {
	ib := bookmark.New()
	ib.URL = app.Info.URL
	ib.Title = app.Info.Title
	ib.Tags = bookmark.ParseTags(app.Info.Tags)
	ib.Desc = app.Info.Desc

	if _, err := r.InsertOne(ctx, ib); err != nil {
		return fmt.Errorf("failed to seed initial bookmark: %w", err)
	}

	p, f := c.Palette(), c.Frame()

	f.Headerln(p.Bold.Sprint("Seeding initial bookmark"))

	u := strings.Replace(ib.URL, "https://", "", 1)

	f.Success(txt.PaddedLineWithPad("inserted", p.Dim.Wrap(u, p.Italic), padding) + "\n").
		Rowln().
		Success("Database initialized\n").
		Success("Setup complete\n").
		Flush()

	return nil
}

func initMigrations(ctx context.Context, app *application.App, c *ui.Console) (*db.SQLite, error) {
	p, f := c.Palette(), c.Frame()

	r, err := db.Init(ctx, app.Path.Database)
	if err != nil {
		return nil, fmt.Errorf("database init failed: %w", err)
	}

	f.Headerln(p.Bold.Sprint("Configuring database"))

	if err := db.Migrate(ctx, r); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	if err := db.UpdateAppVersion(ctx, r, app.Info.Version); err != nil {
		return nil, fmt.Errorf("app version update failed: %w", err)
	}

	schemaVer, err := db.CurrentSchemaVersion(ctx, r)
	if err != nil {
		return nil, err
	}

	sqlVer, err := db.SQLiteVersion(ctx, r)
	if err != nil {
		return nil, err
	}

	f.Success(txt.PaddedLineWithPad("schema version", p.BrightGreen.Sprint(schemaVer)+"\n", padding)).
		Success(txt.PaddedLineWithPad("sqlite version", p.BrightMagenta.Sprint(sqlVer)+"\n", padding)).
		Rowln().
		Flush()

	return r, nil
}
