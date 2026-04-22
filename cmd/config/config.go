// Package config manages the application's configuration, including
// reading from and writing to configuration files (e.g., YAML), and
// providing helper functions for command-line configuration logic.
package config

import (
	"context"
	"fmt"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Flags.JSON {
				return newJSONCmd(app).RunE(cmd, args)
			}

			if app.Flags.Edit {
				return newEditCmd(app).RunE(cmd, args)
			}

			return cmd.Help()
		},
	}

	c.Flags().StringVarP(&app.Flags.ColorStr, "color", "c", "always", "")
	c.Flags().StringVar(&app.DBName, "db", application.MainDBName, "database name")
	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")
	c.Flags().BoolVarP(&app.Flags.Edit, "edit", "e", false, "edit with text editor")
	cmdutil.HideFlag(c, "help", "color", "db")

	c.AddCommand(newCreateCmd(app), newEditCmd(app), newShowPathCmd(app))

	return c
}

func newCreateCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "create configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Validate(); err != nil {
				return err
			}
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			return createConfig(c, app)
		},
	}
}

func newEditCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:     "edit",
		Short:   "edit configuration file",
		Hidden:  true,
		Aliases: []string{"e"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Validate(); err != nil {
				return err
			}

			return editConfig(cmd.Context(), app)
		},
	}
}

func newJSONCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "json",
		Short: "output config in JSON format",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			return printConfigJSON(c, app)
		},
	}
}

func newShowPathCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:    "path",
		Short:  "print config file location",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showPathFile(app.Path.ConfigFile)
		},
	}
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(c *ui.Console, app *application.App) error {
	fn := app.Path.ConfigFile
	if files.Exists(fn) && !app.Flags.Force {
		p := c.Palette()
		f := p.BrightYellow.Wrap("--force", p.Italic, p.Bold)
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, f)
	}

	if !c.Confirm(fmt.Sprintf("create configfile %q", fn), "y") {
		return sys.ErrActionAborted
	}

	if err := application.WriteYAML(fn, app, app.Flags.Force); err != nil {
		return err
	}

	fmt.Fprintf(c.Writer(), "%s: file saved %q\n", app.Name, fn)

	return nil
}

// editConfig edits the config file.
func editConfig(ctx context.Context, app *application.App) error {
	p := app.Path.ConfigFile
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	te, err := editor.NewEditor(app.Env.Editor)
	if err != nil {
		return err
	}

	return te.EditFile(ctx, p)
}

// printConfigJSON prints the config file as JSON.
func printConfigJSON(c *ui.Console, app *application.App) error {
	j, err := port.ToJSON(app)
	if err != nil {
		return err
	}

	fmt.Fprintln(c.Writer(), string(j))

	return nil
}

func showPathFile(p string) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	fmt.Println(p)

	return nil
}
