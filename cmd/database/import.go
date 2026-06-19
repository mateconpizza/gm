package database

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/gitcmd"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func newImportCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "import",
		Aliases:            []string{"imp", "i"},
		Short:              "import bookmarks",
		PersistentPostRunE: cli.HookGitSync(app),
		RunE:               cli.HookHelp,
	}

	c.AddCommand(
		newImportHTMLCmd(app),
		newImportBrowserCmd(app),
		newImportFromDatabaseCmd(app),
		newImportFromBackupCmd(app),
		newImportFromGit(app),
	)

	return c
}

func newImportFromDatabaseCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "database",
		Short:   "import from database",
		Aliases: []string{"db"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Flags.Path != "" {
				return newImportFromFileCmd(app).RunE(cmd, args)
			}

			rDest, err := db.New(cmd.Context(), app.Path.DB())
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			defer rDest.Close()

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			ctx := cmd.Context()

			srcPath, err := handler.SelectDatabase(ctx, d, rDest.Cfg.Fullpath())
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			rSrc, err := db.New(cmd.Context(), srcPath)
			if err != nil {
				return err
			}
			defer rSrc.Close()

			return port.Database(ctx, d, rSrc, rDest)
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "database path")

	return c
}

func newImportFromBackupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "backup",
		Short:   "import from backup",
		Aliases: []string{"bk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			destRepo, err := db.New(cmd.Context(), app.Path.DB())
			if err != nil {
				return err
			}
			defer destRepo.Close()

			dbName := files.StripSuffixes(destRepo.Name())
			bks, err := files.List(app.Path.Backup(), "*_"+dbName+".db*")
			if err != nil {
				return err
			}

			if len(bks) == 0 {
				return db.ErrBackupNotFound
			}

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			ctx := cmd.Context()
			backupPath, err := dbops.SelectBackup(ctx, d, bks)
			if err != nil {
				return err
			}

			srcRepo, err := db.New(ctx, backupPath)
			if err != nil {
				return err
			}
			defer srcRepo.Close()

			return port.FromBackup(ctx, d, destRepo, srcRepo)
		},
	}

	return c
}

func newImportBrowserCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "browser",
		Short: "import from browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return port.Browser(cmd.Context(), d)
		},
	}

	return c
}

func newImportHTMLCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "html",
		Short: "import from HTML Netscape file",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return port.FromHTML(cmd.Context(), d, app.Flags.Path)
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "filename path")
	_ = c.MarkFlagRequired("filename")

	return c
}

func newImportFromFileCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return port.FromFile(cmd.Context(), d, app.Flags.Path)
		},
	}

	return c
}

func newImportFromGit(app *application.App) *cobra.Command {
	var c *cobra.Command

	g := gitcmd.NewCmd(app)
	for _, cmd := range g.Commands() {
		if cmd.Name() == "clone" {
			c = &cobra.Command{
				Use:   "git",
				Short: "import from git repository",
				Args:  cobra.MinimumNArgs(1),
				RunE:  cmd.RunE,
			}
		}
	}

	return c
}
