package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/repository"
)

func initConfig() {
	cfg := config.App
	cfg.Flags.Color = cfg.Flags.ColorStr == "always" && !terminal.IsPiped() && !terminal.NoColorEnv()

	config.SetVerbosity(cfg.Flags.Verbose)

	// load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		sys.ErrAndExit(err)
	}

	// set app home
	config.SetAppPaths(dataHomePath)

	// set database path and name
	cfg.DBName = files.EnsureSuffix(cfg.DBName, ".db")
	cfg.DBPath = filepath.Join(dataHomePath, cfg.DBName)

	// load config from YAML
	if err := loadConfig(cfg.Path.ConfigFile); err != nil {
		slog.Error("loading config", "err", err)
	}

	// set menu
	menu.SetConfig(config.Fzf)

	// enable global color
	menu.ColorEnable(cfg.Flags.Color)
	color.Enable(cfg.Flags.Color)

	// terminal interactive mode
	terminal.NonInteractiveMode(cfg.Flags.Force)

	// git config
	git.Config(cfg)
}

// init sets the config for the root command.
func init() {
	initRootFlags(Root)
	Root.AddCommand(InitCmd)
	cobra.OnInitialize(initConfig)
}

var InitCmd = &cobra.Command{
	Use:    "init",
	Short:  "Initialize a new bookmarks database",
	Hidden: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		if files.Exists(config.App.DBPath) {
			if ok, _ := db.IsInitialized(config.App.DBPath); ok {
				return db.ErrDBExistsAndInit
			}

			return fmt.Errorf("%q %w", config.App.DBName, db.ErrDBExists)
		}

		return nil
	},
	RunE:     initAppFunc,
	PostRunE: initPostFunc,
}

// createPaths creates the paths for the application.
func createPaths(c *ui.Console, path string) error {
	if files.Exists(path) {
		return nil
	}

	ci := color.StyleItalic
	c.F.Headerln(prettyVersion()).Rowln().
		Info(txt.PaddedLine("Create path:", ci(path).Italic().String())).Ln().
		Info(txt.PaddedLine("Create db:", ci(config.App.DBPath).Italic().String())).Ln()

	lines := txt.CountLines(c.F.String()) + 1
	c.F.Rowln().Flush()

	if err := c.ConfirmErr("continue?", "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	// clean terminal keeping header+row
	headerN := 3
	lines += txt.CountLines(c.F.String()) - headerN
	c.ClearLine(lines)

	if err := files.MkdirAll(path); err != nil {
		sys.ErrAndExit(err)
	}

	c.Success(fmt.Sprintf("Created directory path %q\n", path)).Flush()
	c.Success("Inserted initial bookmark\n").Row("\n").Flush()

	return nil
}

// loadDataPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, uses the data user
// directory.
func loadDataPath() (string, error) {
	e := config.App.Env.Home

	envDataHome := sys.Env(e, "")
	if envDataHome != "" {
		slog.Debug("reading home env", e, envDataHome)

		return config.PathJoin(envDataHome), nil
	}

	dataHome, err := config.DataPath()
	if err != nil {
		return "", fmt.Errorf("loading paths: %w", err)
	}

	slog.Debug("home app", "path", dataHome)

	return dataHome, nil
}

// prettyVersion formats version in a pretty way.
func prettyVersion() string {
	name := color.BrightBlue(config.App.Name).Bold().String()
	return fmt.Sprintf("%s v%s %s/%s", name, config.App.Info.Version, runtime.GOOS, runtime.GOARCH)
}

func initAppFunc(_ *cobra.Command, _ []string) error {
	cfg := config.App
	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(
			terminal.New(
				terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(sys.ErrActionAborted) }),
			),
		),
	)

	if err := createPaths(c, cfg.Path.Data); err != nil {
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

	// if err := store.Init(context.Background()); err != nil {
	if err := dbtask.Init(context.Background(), store); err != nil {
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
	ib.Tags = parser.Tags(cfg.Info.Tags)
	ib.Desc = cfg.Info.Desc

	// FIX: opening multiple conn
	store.Close()
	r, err := repository.New(store.Cfg.Fullpath())
	if err != nil {
		return err
	}

	if _, err := r.InsertOne(context.Background(), ib); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.Frame(ib))
	fmt.Print("\n" + c.SuccessMesg("initialized database "+cfg.DBName+"\n"))

	return nil
}

// initPostFunc ask user to track new database if git is initialized.
func initPostFunc(_ *cobra.Command, _ []string) error {
	cfg := config.App
	if !git.IsInitialized(cfg.Git.Path) {
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
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
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
