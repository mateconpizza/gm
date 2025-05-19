package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
	Repo     = repo.SQLiteRepository
)

var (
	// SQLiteCfg holds the configuration for the database and backups.
	Cfg *repo.SQLiteCfg

	// Main database name.
	DBName string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Info.Title,
	Long:         config.App.Info.Desc,
	Version:      prettyVersion(),
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// ignore if one of this subcommands was called.
		subcmds := []string{"init", "new", "version", "lock", "unlock"}
		if isSubCmdCalled(cmd, subcmds...) {
			return nil
		}
		if err := handler.CheckDBNotEncrypted(); err != nil {
			return fmt.Errorf("%w", err)
		}

		return handler.ValidateDBExistence(cmd, Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return recordsCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}
