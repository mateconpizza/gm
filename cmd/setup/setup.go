// Package setup provides commands for initializing and configuring the bookmark database.
// It handles database creation, initial schema setup, and optional Git repository tracking.
package setup

import (
	"fmt"

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

var InitCmd = &cobra.Command{
	Use:               "init",
	Short:             "initialize a new bookmarks database",
	Hidden:            true,
	Annotations:       cli.SkipDBCheckAnnotation,
	PersistentPreRunE: cli.HookCheckIfDatabaseInitialized,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: convert `InitCmd` to command builder(*application.App)
		app, err := application.FromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		return initializeAction(deps.New(cmd.Context(),
			deps.WithApplication(app),
			deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
		))
	},
	PostRunE: InitAppPostFunc,
}

func NewCmd() *cobra.Command {
	return InitCmd
}

func initializeAction(d *deps.Deps) error {
	c, app := d.Console(), d.App
	if err := createPaths(c, app); err != nil {
		return err
	}

	store, err := db.Init(app.DBPath)
	if store == nil {
		return fmt.Errorf("%w", err)
	}
	defer store.Close()

	if ok := store.IsInitialized(d.Context()); ok && !app.Flags.Force {
		return fmt.Errorf("%q %w", store.Name(), db.ErrDBAlreadyInitialized)
	}

	if err := store.Init(d.Context()); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if app.DBName != application.MainDBName {
		fmt.Fprintln(d.Writer(), c.SuccessMesg("initialized database "+app.DBName))
		return nil
	}

	// initial bookmark
	ib := bookmark.New()
	ib.ID = 1
	ib.URL = app.Info.URL
	ib.Title = app.Info.Title
	ib.Tags = bookmark.ParseTags(app.Info.Tags)
	ib.Desc = app.Info.Desc

	if _, err := store.InsertOne(d.Context(), ib); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Fprint(d.Writer(), txt.Frame(c, ib))
	fmt.Fprintln(d.Writer(), "\n"+c.SuccessMesg("initialized database "+app.DBName))

	return nil
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
	gr, err := git.NewRepo(app.DBPath)
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

// createPaths creates the paths for the application.
func createPaths(c *ui.Console, app *application.App) error {
	if files.Exists(app.Path.Data) {
		return nil
	}

	p, f := c.Palette(), c.Frame()
	f.Headerln(cli.PrettyVersion(app.Name, app.Info.Version)).Rowln().
		Info(txt.PaddedLine("Create path:", p.Italic.Sprint(app.Path.Data))).Ln().
		Info(txt.PaddedLine("Create db:", p.Italic.Sprint(app.DBPath))).Ln()

	lines := txt.CountLines(f.String()) + 1
	f.Rowln().Flush()

	if err := c.ConfirmErr("continue?", "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	// clean terminal keeping header+row
	headerN := 3
	lines += txt.CountLines(f.String()) - headerN
	c.ClearLine(lines)

	if err := app.CreatePaths(); err != nil {
		sys.ErrAndExit(err)
	}

	c.Success(fmt.Sprintf("Created directory path %q\n", app.Path.Data)).Flush()
	c.Success("Inserted initial bookmark\n").Row("\n").Flush()

	return nil
}
