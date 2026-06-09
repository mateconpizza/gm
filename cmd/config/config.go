// Package config manages the application's configuration, including
// reading from and writing to configuration files (e.g., YAML), and
// providing helper functions for command-line configuration logic.
package config

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/files"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg", "conf"},
		Short:   "configuration",
		Example: app.Example(`  $ {cmd} config create
  $ {cmd} config create --force
  $ {cmd} config edit
  $ {cmd} config --print
  $ {cmd} config --json`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Flags.JSON {
				return cfgToJSON(cmd.Context(), app)
			}
			if app.Flags.Print {
				return showPathFile(os.Stdout, app.Path.ConfigFile())
			}

			return cmd.Help()
		},
	}
	c.Flags().BoolVar(&app.Flags.Print, "print", false, "print config file location")
	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "JSON output")
	cmdutil.HideFlag(c, "color", "db", "menu", "head", "tail", "fields", "sort", "tag")

	c.AddCommand(newCreateCmd(app), newEditCmd(app))

	return c
}

func newCreateCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "create",
		Short: "create configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Validate(); err != nil {
				return err
			}
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			return createConfig(cmd.Context(), c, app)
		},
	}
	cmdutil.HideFlag(c, "db")
	return c
}

func newEditCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:     "edit",
		Short:   "edit configuration file with text editor",
		Aliases: []string{"e"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Validate(); err != nil {
				return err
			}

			return editConfig(cmd.Context(), app)
		},
	}
}

func cfgToJSON(ctx context.Context, app *application.App) error {
	app.DBName = strings.TrimSuffix(app.DBName, ".db")

	j, err := port.ToJSON(app)
	if err != nil {
		return err
	}
	c := ui.NewDefaultConsole(ctx, func(err error) { sys.ErrAndExit(err) })
	return c.Term().Print(ctx, string(j))
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(ctx context.Context, c *ui.Console, app *application.App) error {
	cfgFile := app.Path.ConfigFile()
	if files.Exists(cfgFile) && !app.Flags.Force {
		p := c.Palette()
		f := p.BrightYellow.Wrap("--force", p.Italic, p.Bold)
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, f)
	}

	if !app.Flags.Force && !c.Confirm(ctx, fmt.Sprintf("create configfile %q", cfgFile), "y") {
		return sys.ErrActionAborted
	}

	if err := app.WriteConfig(app.Flags.Force); err != nil {
		return err
	}

	fmt.Fprintf(c.Writer(), "%s: file saved %q\n", app.Name, cfgFile)

	return nil
}

// editConfig edits the config file.
func editConfig(ctx context.Context, app *application.App) error {
	p := app.Path.ConfigFile()
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	te, err := editor.NewEditor(app.Env.Editor)
	if err != nil {
		return err
	}

	return te.EditFile(ctx, p)
}

func showPathFile(w io.Writer, p string) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	fmt.Fprintln(w, p)

	return nil
}
