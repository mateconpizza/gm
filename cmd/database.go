package cmd

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// dbCmd database management.
var dbCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "Database management",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// ignore if one of this subcommands was called.
		if isSubCmdCalled(cmd, []string{"new", "info"}...) {
			return nil
		}
		p, err := handler.FindDB(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return handler.ValidateDBExists(p)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

// databaseNewCmd initialize a new bookmarks database.
var databaseNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Initialize a new bookmarks database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if DBName == "" {
			return fmt.Errorf("%w: missing database name", repo.ErrDBNameRequired)
		}
		if config.App.DBName == config.DefaultDBName {
			slog.Warn("default database name is reserved", "name", DBName)
			return cmd.Help()
		}
		if initCmd.PersistentPreRunE != nil {
			if err := initCmd.PersistentPreRunE(cmd, args); err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		return initCmd.RunE(cmd, args)
	},
}

// databaseDropCmd drops a database.
var databaseDropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Drop a database",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		return handler.DroppingDB(r, t)
	},
}

// databaseListCmd lists the available databases.
var databaseListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List databases",
	Aliases: []string{"ls", "l"},
	RunE: func(_ *cobra.Command, _ []string) error {
		return handler.ListDatabases(config.App.Path.Data)
	},
}

// databaseInfoCmd shows information about a database.
var databaseInfoCmd = &cobra.Command{
	Use:     "info",
	Short:   "Show information about a database",
	Aliases: []string{"i", "show"},
	RunE: func(_ *cobra.Command, _ []string) error {
		return handler.RepoInfo(config.App.DBPath, JSON)
	},
}

// databaseRmCmd remove a database.
var databaseRmCmd = &cobra.Command{
	Use:     "rm",
	Short:   "Remove a database",
	Aliases: []string{"r", "remove"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbRemoveCmd.RunE(cmd, args)
	},
}

var databaseLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock a database",
	RunE: func(_ *cobra.Command, _ []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
		return handler.LockRepo(t, config.App.DBPath)
	},
}

var databaseUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
		r := filepath.Join(config.App.Path.Data, config.App.DBName)
		return handler.UnlockRepo(t, r)
	},
}

func init() {
	f := dbCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	_ = dbCmd.Flags().MarkHidden("color")

	// new database
	databaseNewCmd.Flags().StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	// show database info
	databaseInfoCmd.Flags().BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	// remove database
	databaseRmCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "select database to remove (fzf)")
	// add subcommands
	dbCmd.AddCommand(
		databaseDropCmd, databaseInfoCmd, databaseNewCmd, databaseListCmd,
		databaseRmCmd, databaseLockCmd, databaseUnlockCmd,
	)
	rootCmd.AddCommand(dbCmd)
}
