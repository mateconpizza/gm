package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// DBName main database name.
var DBName string

func initConfig() {
	config.SetVerbosity(VerboseFlag)
	config.EnableColor(WithColor == "always" && !terminal.IsPiped() && !terminal.NoColorEnv())
	config.SetForce(Force)

	// load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		sys.ErrAndExit(err)
	}

	// set app home
	config.SetPaths(dataHomePath)
	// set colorscheme path
	config.SetColorSchemePath(filepath.Join(dataHomePath, "colorscheme"))
	// set database name
	config.SetDBName(files.EnsureExt(DBName, ".db"))
	// set database path
	config.SetDBPath(filepath.Join(dataHomePath, config.App.DBName))

	// load config from YAML
	if err := loadConfig(config.App.Path.ConfigFile); err != nil {
		slog.Error("loading config", "err", err)
	}
	menu.SetConfig(config.Fzf)

	// enable color in menu UI
	menu.EnableColor(config.App.Color)

	// enable global color
	color.Enable(config.App.Color)
}

// init sets the config for the root command.
func init() {
	initRootFlags(rootCmd)
	rootCmd.AddCommand(initCmd)
	cobra.OnInitialize(initConfig)
}

// createPaths creates the paths for the application.
func createPaths(t *terminal.Term, path string) error {
	if files.Exists(path) {
		return nil
	}
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(prettyVersion()).Ln().Row().Ln()
	p := color.Text(path).Italic().String()
	fp := color.Text(config.App.DBPath).Italic().String()
	f.Info(format.PaddedLine("Create path:", p+"\n"))
	f.Info(format.PaddedLine("Create db:", fp+"\n"))
	lines := format.CountLines(f.String()) + 1
	f.Row("\n").Flush()
	if err := t.ConfirmErr(f.Question("continue?").String(), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}
	// clean terminal keeping header+row
	headerN := 3
	lines += format.CountLines(f.String()) - headerN
	t.ClearLine(lines)
	if err := files.MkdirAll(path); err != nil {
		sys.ErrAndExit(err)
	}
	f.Clear().Success(fmt.Sprintf("Created directory path %q\n", path))
	f.Success("Inserted initial bookmark\n").Row("\n").Flush()

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

var initCmd = &cobra.Command{
	Use:    "init",
	Short:  "Initialize a new bookmarks database",
	Hidden: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		if files.Exists(config.App.DBPath) {
			if ok, _ := repo.IsInitialized(config.App.DBPath); ok {
				return repo.ErrDBExistsAndInit
			}

			return fmt.Errorf("%q %w", config.App.DBName, repo.ErrDBExists)
		}

		return nil
	},
	RunE: func(_ *cobra.Command, _ []string) error {
		// create paths for the application.
		t := terminal.New()
		if err := createPaths(t, config.App.Path.Data); err != nil {
			return err
		}
		// init database
		r, err := repo.Init(config.App.DBPath)
		if r == nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		// initialize database
		if r.IsInitialized() && !config.App.Force {
			return fmt.Errorf("%q %w", r.Name(), repo.ErrDBAlreadyInitialized)
		}
		if err := r.Init(); err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}
		// ignore initial bookmark if not DefaultDBName
		if config.App.DBName != config.DefaultDBName {
			s := color.Gray(config.App.DBName).Italic().String()
			success := color.BrightGreen("Successfully").Italic().String()
			fmt.Println(success + " initialized database " + s)

			return nil
		}
		// initial bookmark
		ib := bookmark.New()
		ib.URL = config.App.Info.URL
		ib.Title = config.App.Info.Title
		ib.Tags = bookmark.ParseTags(config.App.Info.Tags)
		ib.Desc = config.App.Info.Desc
		// insert new bookmark
		if err := r.InsertOne(context.Background(), ib); err != nil {
			return fmt.Errorf("%w", err)
		}
		// print new record
		fmt.Print(bookmark.Frame(ib, color.DefaultColorScheme()))
		s := color.BrightGreen("Successfully").Italic().String()
		mesg := s + " initialized database " + color.Gray(config.App.DBName+"\n").Italic().String()
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Row("\n").Success(mesg).Flush()

		return nil
	},
}

// prettyVersion formats version in a pretty way.
func prettyVersion() string {
	name := color.BrightBlue(config.App.Name).Bold().String()
	return fmt.Sprintf("%s v%s %s/%s", name, config.App.Info.Version, runtime.GOOS, runtime.GOARCH)
}
