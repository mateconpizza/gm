package database

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/gitcmd"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
)

func newImportCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "import",
		Aliases:            []string{"imp", "i"},
		Short:              "import bookmarks",
		PersistentPostRunE: cli.HookGitSync(app),
	}

	c.AddCommand(
		newImportHTMLCmd(app),
		newImportBrowserCmd(app),
		newImportFromDatabaseCmd(app),
		newImportFromBackupCmd(app),
		newImportFromGit(app),
		newImportFromJSON(app),
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
			return cmdutil.Run(cmd, args, port.ImportFromDatabase)
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "database path")

	return c
}

func newImportFromBackupCmd(_ *application.App) *cobra.Command {
	return &cobra.Command{
		Use:     "backup",
		Short:   "import from backup",
		Aliases: []string{"bk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, port.ImportFromBackup)
		},
	}
}

func newImportBrowserCmd(_ *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "browser",
		Short: "import from browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, port.ImportFromBrowser)
		},
	}
}

func newImportHTMLCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "html",
		Short: "import from HTML Netscape file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, func(ctx context.Context, d *deps.Deps) error {
				return port.ImportFromHTML(cmd.Context(), d, app.Flags.Path)
			})
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "filename path")
	_ = c.MarkFlagRequired("filename")

	return c
}

func newImportFromFileCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, func(ctx context.Context, d *deps.Deps) error {
				return port.ImportFromDatabasePath(cmd.Context(), d, app.Flags.Path)
			})
		},
	}
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

func newImportFromJSON(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "json",
		Short: "import from JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, func(ctx context.Context, d *deps.Deps) error {
				return port.ImportFromJSON(cmd.Context(), d, app.Flags.Path)
			})
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "filename path")
	_ = c.MarkFlagRequired("filename")

	return c
}
