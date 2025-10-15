// Package setup provides commands for initializing and configuring the bookmark database.
// It handles database creation, initial schema setup, and optional Git repository tracking.
package setup

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
	RunE:              InitAppFunc,
	PostRunE:          InitAppPostFunc,
}

func NewCmd() *cobra.Command {
	return InitCmd
}

func InitAppFunc(cmd *cobra.Command, _ []string) error {
	cfg := config.New()
	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(
			terminal.WithContext(cmd.Context()),
			terminal.WithInterruptFn(func(err error) {
				sys.ErrAndExit(sys.ErrActionAborted)
			})),
		),
	)

	if err := createPaths(c, cfg); err != nil {
		return err
	}

	store, err := db.Init(cfg.DBPath)
	if store == nil {
		return fmt.Errorf("%w", err)
	}
	defer store.Close()

	// if store.IsInitialized() && !cfg.Flags.Force {
	if dbtask.IsInitialized(store) && !cfg.Flags.Force {
		return fmt.Errorf("%q %w", store.Name(), db.ErrDBAlreadyInitialized)
	}

	if err := dbtask.Init(cmd.Context(), store); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if cfg.DBName != config.MainDBName {
		fmt.Println(c.SuccessMesg("initialized database " + cfg.DBName))

		return nil
	}

	// initial bookmark
	ib := bookmark.New()
	ib.ID = 1
	ib.URL = cfg.Info.URL
	ib.Title = cfg.Info.Title
	ib.Tags = bookmark.ParseTags(cfg.Info.Tags)
	ib.Desc = cfg.Info.Desc

	// FIX: opening multiple conn
	store.Close()
	r, err := db.New(store.Cfg.Fullpath())
	if err != nil {
		return err
	}

	if _, err := r.InsertOne(cmd.Context(), ib); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.Frame(ib))
	fmt.Print("\n" + c.SuccessMesg("initialized database "+cfg.DBName+"\n"))

	return nil
}

// InitAppPostFunc ask user to track new database if git is initialized.
func InitAppPostFunc(cmd *cobra.Command, _ []string) error {
	cfg := config.New()
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

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(
			terminal.WithContext(cmd.Context()),
			terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
		)),
	)

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

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Loc.DBName)))

	return nil
}

// createPaths creates the paths for the application.
func createPaths(c *ui.Console, cfg *config.Config) error {
	if files.Exists(cfg.Path.Data) {
		return nil
	}

	ci := color.StyleItalic
	c.Frame.Headerln(cli.PrettyVersion(cfg.Name, cfg.Info.Version)).Rowln().
		Info(txt.PaddedLine("Create path:", ci(cfg.Path.Data).Italic().String())).Ln().
		Info(txt.PaddedLine("Create db:", ci(cfg.DBPath).Italic().String())).Ln()

	lines := txt.CountLines(c.Frame.String()) + 1
	c.Frame.Rowln().Flush()

	if err := c.ConfirmErr("continue?", "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	// clean terminal keeping header+row
	headerN := 3
	lines += txt.CountLines(c.Frame.String()) - headerN
	c.ClearLine(lines)

	if err := cfg.CreatePaths(); err != nil {
		sys.ErrAndExit(err)
	}

	c.Success(fmt.Sprintf("Created directory path %q\n", cfg.Path.Data)).Flush()
	c.Success("Inserted initial bookmark\n").Row("\n").Flush()

	return nil
}
