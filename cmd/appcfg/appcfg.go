// Package appcfg manages the application's configuration, including
// reading from and writing to configuration files (e.g., YAML), and
// providing helper functions for command-line configuration logic.
package appcfg

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/files"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg", "conf"},
		Short:   "configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Flags.JSON {
				return newJSONCmd(cfg).RunE(cmd, args)
			}

			return cmd.Help()
		},
	}

	c.Flags().StringVarP(&cfg.Flags.ColorStr, "color", "c", "always", "")
	c.Flags().StringVar(&cfg.DBName, "db", config.MainDBName, "database name")
	c.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	cmdutil.HideFlag(c, "help", "color", "db")

	c.AddCommand(newCreateCmd(cfg), newEditCmd(cfg), newShowPathCmd(cfg))

	return c
}

func newCreateCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "create configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			return createConfig(c, cfg)
		},
	}
}

func newEditCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "edit",
		Short:   "edit configuration file",
		Aliases: []string{"e"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}

			return editConfig(cmd.Context(), cfg)
		},
	}
}

func newJSONCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "json",
		Short: "output config in JSON format",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			return printConfigJSON(c, cfg)
		},
	}
}

func newShowPathCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "print config file location",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showPathFile(cfg.Path.ConfigFile)
		},
	}
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(c *ui.Console, cfg *config.Config) error {
	fn := cfg.Path.ConfigFile
	if files.Exists(fn) && !cfg.Flags.Force {
		p := c.Palette()
		f := p.BrightYellow.Wrap("--force", p.Italic, p.Bold)
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, f)
	}

	if !c.Confirm(fmt.Sprintf("create configfile %q", fn), "y") {
		return sys.ErrActionAborted
	}

	if err := config.WriteYAML(fn, cfg, cfg.Flags.Force); err != nil {
		return err
	}

	fmt.Fprintf(c.Writer(), "%s: file saved %q\n", cfg.Name, fn)

	return nil
}

// editConfig edits the config file.
func editConfig(ctx context.Context, cfg *config.Config) error {
	p := cfg.Path.ConfigFile
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	te, err := editor.NewEditor(cfg.Env.Editor)
	if err != nil {
		return err
	}

	return te.EditFile(ctx, p)
}

// printConfigJSON prints the config file as JSON.
func printConfigJSON(c *ui.Console, cfg *config.Config) error {
	j, err := port.ToJSON(cfg)
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
