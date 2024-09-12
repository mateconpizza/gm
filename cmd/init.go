package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/util"
	"github.com/haaag/gm/pkg/slice"
)

var initCmd = &cobra.Command{
	Use:    "init",
	Short:  "initialize a new bookmarks database",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if r == nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer r.Close()

		if err := initDB(r); err != nil {
			return err
		}

		bs := slice.New[Bookmark]()
		if err := r.GetAll(Cfg.GetTableMain(), bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		// get initial bookmark
		List = true
		// prints bookmark with frame
		Frame = true

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initDB(r *repo.SQLiteRepository) error {
	if r.IsDatabaseInitialized(r.Cfg.GetTableMain()) && !Force {
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

	if _, err := r.Insert(r.Cfg.GetTableMain(), initialBookmark); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(format.Header(prettyVersion(Prettify)))

	return nil
}

// loadDataPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, uses the data user
// directory.
func loadDataPath() (string, error) {
	envDataHome := util.GetEnv(config.App.Env.Home, "")
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
