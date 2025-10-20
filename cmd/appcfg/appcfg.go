// Package appcfg manages the application's configuration, including
// reading from and writing to configuration files (e.g., YAML), and
// providing helper functions for command-line configuration logic.
package appcfg

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/editor"
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
			c := ui.NewDefaultConsole(cmd.Context(), nil)

			switch {
			case cfg.Flags.Create:
				return createConfig(c, cfg)
			case cfg.Flags.Edit:
				return editConfig(cmd.Context(), cfg)
			case cfg.Flags.JSON:
				return printConfigJSON(cfg.Path.ConfigFile)
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
	f.BoolVarP(&cfg.Flags.List, "show", "l", false, "current config filepath")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	f.BoolVar(&cfg.Flags.Force, "force", false, "force action")

	return configCmd
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(c *ui.Console, cfg *config.Config) error {
	p := cfg.Path.ConfigFile
	if files.Exists(p) && !cfg.Flags.Force {
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, c.Palette().BrightYellowItalic("--force"))
	}

	if !c.Confirm(fmt.Sprintf("create configfile %q", p), "y") {
		return nil
	}

	if cfg.Git.Enabled {
		config.Defaults.Git = cfg.Git
	}

	if err := writeYAML(p, config.Defaults, cfg.Flags.Force); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("%s: file saved %q\n", cfg.Name, p)

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

// getConfig loads the config file.
func getConfig(p string) (*config.ConfigFile, error) {
	if !files.Exists(p) {
		return nil, fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	var cfg *config.ConfigFile
	if err := readYAML(p, &cfg); err != nil {
		return nil, fmt.Errorf("reading configile: %w", err)
	}

	if cfg == nil {
		return nil, fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return cfg, nil
}

// printConfigJSON prints the config file as JSON.
func printConfigJSON(p string) error {
	cfg, err := getConfig(p)
	if err != nil {
		return err
	}

	j, err := port.ToJSON(cfg)
	if err != nil {
		return err
	}

	fmt.Println(string(j))

	return nil
}

// writeYAML writes the provided YAML data to the specified file.
func writeYAML[T any](p string, v *T, force bool) error {
	f, err := files.Touch(p, force)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Yaml closing file", "file", p, "error", err)
		}
	}()

	data, err := yaml.Marshal(&v)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	slog.Info("YamlWrite success", "path", p)

	return nil
}

// readYAML unmarshals the YAML data from the specified file.
func readYAML[T any](p string, v *T) error {
	if !files.Exists(p) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, p)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = yaml.Unmarshal(content, &v)
	if err != nil {
		return fmt.Errorf("error unmarshalling YAML: %w", err)
	}

	slog.Debug("YamlRead", "path", p)

	return nil
}

func showPathFile(p string) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	fmt.Println(p)

	return nil
}
