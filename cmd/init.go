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
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var dumpConfig bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new bookmarks database",
	RunE: func(_ *cobra.Command, _ []string) error {
		if dumpConfig {
			if err := menu.DumpConfig(Force); err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		}
		// create paths for the application.
		p := config.App.Path
		t := terminal.New()
		if err := createPaths(t, p.Data); err != nil {
			return err
		}
		// init database
		r, err := repo.New(Cfg)
		if r == nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer r.Close()
		b, err := initDB(r)
		if err != nil {
			return err
		}
		// print new record
		fmt.Print(bookmark.Frame(b))
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
func createPaths(t *terminal.Term, path string) error {
	f := frame.New(frame.WithColorBorder(color.Gray), frame.WithNoNewLine())
	f.Header(prettyVersion()).Ln().Row().Ln()
	if files.Exists(path) {
		return nil
	}

	f.Mid(format.PaddedLine("create path:", fmt.Sprintf("'%s'", path))).Ln()
	f.Mid(format.PaddedLine("create db:", fmt.Sprintf("'%s'", Cfg.Fullpath()))).Ln()
	f.Row().Ln().Render()
	lines := format.CountLines(f.String())
	if !t.Confirm(f.Clean().Footer("continue?").String(), "y") {
		return handler.ErrActionAborted
	}
	// clean terminal keeping header+row
	headerN := 3
	lines += format.CountLines(f.String()) - headerN
	t.ClearLine(lines)
	if err := files.MkdirAll(path); err != nil {
		sys.ErrAndExit(err)
	}
	f.Clean()
	f.Success(fmt.Sprintf("Successfully created directory path '%s'.", path)).Ln()
	f.Success("Successfully created initial bookmark.").Ln().Row().Ln().Render()

	return nil
}

// initDB creates a new database and populates it with the initial bookmark.
func initDB(r *repo.SQLiteRepository) (*Bookmark, error) {
	if r.IsInitialized() && !Force {
		return nil, fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyInitialized, DBName)
	}
	if err := r.Init(); err != nil {
		return nil, fmt.Errorf("initializing database: %w", err)
	}
	// initial bookmark
	ib := bookmark.New()
	ib.URL = config.App.Info.URL
	ib.Title = config.App.Info.Title
	ib.Tags = bookmark.ParseTags(config.App.Info.Tags)
	ib.Desc = config.App.Info.Desc
	// insert new bookmark
	if err := r.Insert(ib); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return ib, nil
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
