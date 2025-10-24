// Package appcfg manages the application's configuration, including
// reading from and writing to configuration files (e.g., YAML), and
// providing helper functions for command-line configuration logic.
package appcfg

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/files"
)

func NewCmd() *cobra.Command {
	cfg := config.New()

	configCmd := &cobra.Command{
		Use:     "conf",
		Aliases: []string{"c", "config"},
		Short:   "Configuration management",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })

			switch {
			case cfg.Flags.Create:
				return createConfig(c, cfg)
			case cfg.Flags.Edit:
				return editConfig(cmd.Context(), cfg)
			case cfg.Flags.JSON:
				return printConfigJSON(c, cfg)
			case cfg.Flags.List:
				return showPathFile(cfg.Path.ConfigFile)
			}

			return cmd.Usage()
		},
	}

	f := configCmd.Flags()
	f.SortFlags = false
	f.BoolVarP(&cfg.Flags.Create, "create", "c", false, "create config file")
	f.BoolVarP(&cfg.Flags.Edit, "edit", "e", false, "edit config")
	f.BoolVarP(&cfg.Flags.List, "show-path", "s", false, "display config file location")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	return configCmd
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(c *ui.Console, cfg *config.Config) error {
	p := cfg.Path.ConfigFile
	if files.Exists(p) && !cfg.Flags.Force {
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, c.Palette().BrightYellowItalic("--force"))
	}

	if !c.Confirm(fmt.Sprintf("create configfile %q", p), "y") {
		return sys.ErrActionAborted
	}

	if err := config.WriteYAML(p, cfg, cfg.Flags.Force); err != nil {
		return err
	}

	fmt.Fprintf(c.Writer(), "%s: file saved %q\n", cfg.Name, p)

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
		return fmt.Errorf("%w", err)
	}

	if err := te.EditFile(ctx, p); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
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
