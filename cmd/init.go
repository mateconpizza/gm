package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
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

		var bs = bookmark.NewSlice[Bookmark]()
		if err := r.GetAll(Cfg.GetTableMain(), bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		Prettify = true
		return handleFormat(bs)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initDB(r *Repo) error {
	if r.IsInitialized(r.Cfg.GetTableMain()) && !Force {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyInitialized, DBName)
	}
	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	initialBookmark := bookmark.New(
		App.Info.URL,
		App.Info.Title,
		format.ParseTags(App.Info.Tags),
		App.Info.Desc,
	)

	printSummary()

	if _, err := r.Insert(r.Cfg.GetTableMain(), initialBookmark); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func printSummary() {
	fmt.Println(app.PrettyVersion(Prettify))
	fmt.Printf("+ app folder at: %s\n", C(App.Home).Yellow())
	fmt.Printf("+ %s folder at: %s\n", C("databases").Blue(), C(Cfg.GetHome()).Cyan())
	fmt.Printf("+ database '%s' initialized\n", C(DBName).Green())
	fmt.Printf("+ %s bookmark created\n\n", C("initial").Purple())
}
