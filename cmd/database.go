package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrDBNameRequired = errors.New("name required")

var databaseNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Initialize a new bookmarks database",
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		t := terminal.New()
		if len(args) == 0 {
			name = t.Prompt("+ New database name: ")
		} else {
			name = args[0]
		}
		if name == "" {
			return ErrDBNameRequired
		}
		if err := handler.ValidateDBExistence(cmd, Cfg); err != nil {
			return fmt.Errorf("%w", err)
		}
		Cfg.SetName(name)

		return initCmd.RunE(cmd, args)
	},
}

// databaseDropCmd drops a database.
var databaseDropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Drop a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		if !r.IsInitialized() && !config.App.Force {
			return fmt.Errorf("%w: %q", repo.ErrDBNotInitialized, r.Name())
		}
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		f := frame.New(frame.WithColorBorder(color.BrightGray))
		warn := color.BrightRed("dropping").String()
		f.Header(warn + " all bookmarks database").Ln().Row().Ln().Flush()
		fmt.Print(repo.Info(r))
		f.Clear().Row().Ln().Flush().Clear()
		if !config.App.Force {
			if !t.Confirm(f.Question("continue?").String(), "n") {
				return sys.ErrActionAborted
			}
		}

		if err := r.DropSecure(context.Background()); err != nil {
			return fmt.Errorf("%w", err)
		}

		if VerboseFlag > 0 {
			t.ClearLine(1)
		}
		success := color.BrightGreen("Successfully").Italic().String()
		f.Clear().Success(success + " database dropped").Ln().Flush()

		return nil
	},
}

// dbDropCmd drops a database.
var databaseListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List databases",
	Aliases: []string{"ls", "l"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		// unlocked databases
		dbs, err := repo.Databases(r.Cfg.Path)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		n := dbs.Len()
		if n == 0 {
			return fmt.Errorf("%w", repo.ErrDBsNotFound)
		}
		// locked databases
		s, err := repo.DatabasesEncrypted(config.App.Path.Data)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		f := frame.New(frame.WithColorBorder(color.BrightGray))
		// add header
		if n > 1 || len(s) > 1 {
			n += len(s)
			nColor := color.BrightCyan(n).Bold().String()
			f.Header(nColor + " database/s found\n").Row("\n")
		}

		dbs.ForEachMut(func(r *Repo) {
			f.Text(repo.RepoSummary(r))
		})

		if len(s) > 0 {
			for _, file := range s {
				file = strings.TrimSuffix(file, ".enc")
				s := color.BrightMagenta(filepath.Base(file)).Italic().String()
				e := color.BrightGray("(encrypted) ").Italic().String()
				f.Mid(format.PaddedLine(s, e)).Ln()
			}
		}

		f.Flush()

		return nil
	},
}

// databaseInfoCmd shows information about a database.
var databaseInfoCmd = &cobra.Command{
	Use:     "info",
	Short:   "Show information about a database",
	Aliases: []string{"i", "show"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		r.Cfg.BackupFiles, _ = r.BackupsList()
		if JSON {
			fmt.Println(string(format.ToJSON(r)))

			return nil
		}

		fmt.Print(repo.Info(r))

		return nil
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
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		r := filepath.Join(config.App.Path.Data, config.App.DBName)

		return handler.LockDB(t, r)
	},
}

var databaseUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		r := filepath.Join(config.App.Path.Data, config.App.DBName)

		return handler.UnlockDB(t, r)
	},
}

// dbCmd database management.
var dbCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "Database management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

func init() {
	f := dbCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	databaseInfoCmd.Flags().BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	_ = dbCmd.Flags().MarkHidden("color")
	// add subcommands
	dbCmd.AddCommand(
		databaseDropCmd,
		databaseInfoCmd,
		databaseNewCmd,
		databaseListCmd,
		databaseRmCmd,
		databaseLockCmd,
		databaseUnlockCmd,
	)
	rootCmd.AddCommand(dbCmd)
}
