package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var dumpConfig bool

var initCmd = &cobra.Command{
	Use:    "init",
	Short:  "initialize a new bookmarks database",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if dumpConfig {
			if err := menu.DumpConfig(Force); err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		}

		// create paths for the application.
		p := config.App.Path
		if err := createPaths(p.Data); err != nil {
			return err
		}

		// init database
		r, err := repo.New(Cfg)
		if r == nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer r.Close()

		if err := initDB(r); err != nil {
			return err
		}

		// print new record
		bs := slice.New[Bookmark]()
		if err := r.Records(Cfg.Tables.Main, bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		if err := handler.Print(bs); err != nil {
			return fmt.Errorf("initCmd printer: %w", err)
		}

		f := frame.New(frame.WithColorBorder(color.Gray))
		s := color.Gray(Cfg.Name).Italic().String()
		f.Row().Success("Successfully initialized database " + s).Render()

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&dumpConfig, "dump-config", false, "dump config data")
	rootCmd.AddCommand(initCmd)
}

// createPaths creates the paths for the application.
func createPaths(path string) error {
	f := frame.New(frame.WithColorBorder(color.Gray), frame.WithNoNewLine())
	f.Header(prettyVersion()).Ln()
	f.Row().Ln()

	if files.Exists(path) {
		return nil
	}

	f.Mid(format.PaddedLine("create path:", fmt.Sprintf("'%s'", path))).Ln()
	f.Mid(format.PaddedLine("create db:", fmt.Sprintf("'%s'", Cfg.Fullpath()))).Ln()
	f.Render()

	lines := format.CountLines(f.String())

	q := f.Clean().Row().Ln().Footer("continue?").String()
	if !terminal.Confirm(q, "y") {
		return terminal.ErrActionAborted
	}

	// clean terminal keeping header+row
	headerN := 3
	lines += format.CountLines(f.String()) - headerN
	terminal.ClearLine(lines)

	if err := files.MkdirAll(path); err != nil {
		sys.ErrAndExit(err)
	}

	f.Clean()
	f.Success(fmt.Sprintf("Successfully created directory path '%s'.", path)).Ln()
	f.Success("Successfully created initial bookmark.").Ln()
	f.Row().Ln()
	f.Render()

	return nil
}

// initDB creates a new database and populates it with the initial bookmark.
func initDB(r *repo.SQLiteRepository) error {
	if r.IsInitialized() && !Force {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyInitialized, DBName)
	}

	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	// initial bookmark
	ib := bookmark.New()
	ib.URL = config.App.Info.URL
	ib.Title = config.App.Info.Title
	ib.Tags = bookmark.ParseTags(config.App.Info.Tags)
	ib.Desc = config.App.Info.Desc

	if err := r.Insert(ib); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// loadDataPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, uses the data user
// directory.
func loadDataPath() (string, error) {
	envDataHome := sys.Env(config.App.Env.Home, "")
	if envDataHome != "" {
		log.Printf("loadPath: envDataHome: %v\n", envDataHome)

		return config.PathJoin(envDataHome), nil
	}
	dataHome, err := config.DataPath()
	if err != nil {
		return "", fmt.Errorf("loading paths: %w", err)
	}
	log.Printf("loadPath: dataHome: %v\n", dataHome)

	return dataHome, nil
}
