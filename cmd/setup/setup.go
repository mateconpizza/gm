// Package setup provides commands for initializing and configuring the bookmark database.
// It handles database creation, initial schema setup, and optional Git repository tracking.
package setup

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
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
	Short:             "Initialize a new bookmarks database",
	Hidden:            true,
	Annotations:       cli.SkipDBCheckAnnotation,
	PersistentPreRunE: cli.HookCheckIfDatabaseInitialized,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.FromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		return initializeAction(app.New(cmd.Context(),
			app.WithConfig(cfg),
			app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
		))
	},
	PostRunE: InitAppPostFunc,
}

func NewCmd() *cobra.Command {
	return InitCmd
}

func initializeAction(a *app.Context) error {
	c, cfg := a.Console(), a.Cfg
	if err := createPaths(c, cfg); err != nil {
		return err
	}

	store, err := db.Init(cfg.DBPath)
	if store == nil {
		return fmt.Errorf("%w", err)
	}
	defer store.Close()

	if ok := store.IsInitialized(a.Context()); ok && !cfg.Flags.Force {
		return fmt.Errorf("%q %w", store.Name(), db.ErrDBAlreadyInitialized)
	}

	if err := store.Init(a.Context()); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if cfg.DBName != config.MainDBName {
		fmt.Fprintln(a.Writer(), c.SuccessMesg("initialized database "+cfg.DBName))
		return nil
	}

	// initial bookmark
	ib := bookmark.New()
	ib.ID = 1
	ib.URL = cfg.Info.URL
	ib.Title = cfg.Info.Title
	ib.Tags = bookmark.ParseTags(cfg.Info.Tags)
	ib.Desc = cfg.Info.Desc

	if _, err := store.InsertOne(a.Context(), ib); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Fprint(a.Writer(), txt.Frame(c, ib))
	fmt.Fprintln(a.Writer(), "\n"+c.SuccessMesg("initialized database "+cfg.DBName))

	return nil
}

// InitAppPostFunc ask user to track new database if git is initialized.
func InitAppPostFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !cfg.Git.Enabled {
		return nil
	}
	gr, err := git.NewRepo(cfg.DBPath)
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
func createPaths(c *ui.Console, cfg *config.Config) error {
	if files.Exists(cfg.Path.Data) {
		return nil
	}

	p, f := c.Palette(), c.Frame()
	f.Headerln(cli.PrettyVersion(cfg.Name, cfg.Info.Version)).Rowln().
		Info(txt.PaddedLine("Create path:", p.Italic.Sprint(cfg.Path.Data))).Ln().
		Info(txt.PaddedLine("Create db:", p.Italic.Sprint(cfg.DBPath))).Ln()

	lines := txt.CountLines(f.String()) + 1
	f.Rowln().Flush()

	if err := c.ConfirmErr("continue?", "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	// clean terminal keeping header+row
	headerN := 3
	lines += txt.CountLines(f.String()) - headerN
	c.ClearLine(lines)

	if err := cfg.CreatePaths(); err != nil {
		sys.ErrAndExit(err)
	}

	c.Success(fmt.Sprintf("Created directory path %q\n", cfg.Path.Data)).Flush()
	c.Success("Inserted initial bookmark\n").Row("\n").Flush()

	return nil
}
