package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
)

var initCmd = &cobra.Command{
	Use:    "init",
	Short:  "initialize a new bookmarks database",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		// Create paths for the application.
		var builder strings.Builder
		p := config.App.Path
		if !files.Exists(p.Data) {
			if err := files.MkdirAll(p.Backup); err != nil {
				logErrAndExit(err)
			}

			builder.WriteString(fmt.Sprintf("\nSuccessfully created directory path '%s'.", p.Data))
			builder.WriteString("\nSuccessfully created initial bookmark.")
		}

		r, err := repo.New(Cfg)
		if r == nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer r.Close()

		if err := initDB(r); err != nil {
			return err
		}

		bs := slice.New[Bookmark]()
		if err := r.Records(Cfg.TableMain, bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		// Get initial bookmark
		List = true
		// Prints bookmark with frame
		Frame = true

		if err := handlePrintOut(bs); err != nil {
			return err
		}

		if builder.Len() > 0 {
			fmt.Println(builder.String())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initDB(r *repo.SQLiteRepository) error {
	if r.IsDatabaseInitialized(r.Cfg.TableMain) && !Force {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyInitialized, DBName)
	}

	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	initialBookmark := bookmark.New(
		config.App.Info.URL,
		config.App.Info.Title,
		bookmark.ParseTags(config.App.Info.Tags),
		config.App.Info.Desc,
	)

	if _, err := r.Insert(r.Cfg.TableMain, initialBookmark); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(format.Header(prettyVersion()))

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
