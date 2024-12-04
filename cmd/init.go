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

		if err := initDB(r); err != nil {
			return err
		}

		// print new record
		bs := slice.New[Bookmark]()
		if err := r.Records(Cfg.TableMain, bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		r.Close()

		if err := handlePrintOut(bs); err != nil {
			return err
		}

		s := color.BrightGray(Cfg.Name).Italic().String()
		fmt.Println("\nSuccessfully initialized database " + s + ".")

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&dumpConfig, "dump-config", false, "dump config data")
	rootCmd.AddCommand(initCmd)
}

func createPaths(path string) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header(prettyVersion()).Ln()
	f.Row().Ln()

	if files.Exists(path) {
		return nil
	}

	f.Mid(format.PaddedLine("create path:", fmt.Sprintf("'%s'", path))).Ln()
	f.Mid(format.PaddedLine("create db:", fmt.Sprintf("'%s'", Cfg.Fullpath()))).Ln()
	f.Row().Ln().Footer("continue?").Render()

	if !terminal.Confirm("", "n") {
		return terminal.ErrActionAborted
	}

	// clean terminal keeping header+row
	headerN := 2
	lines := format.CountLines(f.String()) - headerN
	terminal.ClearLine(lines)

	if err := files.MkdirAll(path); err != nil {
		logErrAndExit(err)
	}

	f.Clean()
	f.Mid(fmt.Sprintf("Successfully created directory path '%s'.", path)).Ln()
	f.Footer("Successfully created initial bookmark.").Ln()
	f.Row().Ln()
	f.Render()

	return nil
}

// initDB creates a new database and populates it with the initial bookmark.
func initDB(r *repo.SQLiteRepository) error {
	if r.IsDatabaseInitialized(r.Cfg.TableMain) && !Force {
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

	if _, err := r.Insert(r.Cfg.TableMain, ib); err != nil {
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
