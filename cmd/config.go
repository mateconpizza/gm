package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/editor"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/files"
)

// createConfFlag is the flag for creating a new config file.
var createConfFlag bool

func init() {
	cfg := config.App
	f := configCmd.Flags()
	f.BoolVarP(&createConfFlag, "create", "c", false, "create config file")
	f.BoolVarP(&cfg.Flags.Edit, "edit", "e", false, "edit config")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	f.StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "database name")
	_ = f.MarkHidden("name")

	Root.AddCommand(configCmd)
}

// configCmd configuration management.
var configCmd = &cobra.Command{
	Use:     "conf",
	Aliases: []string{"c", "config"},
	Short:   "Configuration management",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := config.App
		switch {
		case createConfFlag:
			return createConfig(cfg.Path.ConfigFile)
		case cfg.Flags.Edit:
			return editConfig(cfg.Path.ConfigFile)
		case cfg.Flags.JSON:
			return printConfigJSON(cfg.Path.ConfigFile)
		}

		return cmd.Usage()
	},
}

// createConfig dumps the app configuration to a YAML file.
func createConfig(p string) error {
	if files.Exists(p) && !config.App.Flags.Force {
		f := color.BrightYellow("--force").Italic().String()
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, f)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Info(txt.PaddedLine("File:", color.Text(p).Italic()) + "\n").Row("\n")

	if !terminal.Confirm(f.Question("create?").String(), "y") {
		return nil
	}

	if err := writeYAML(p, config.Defaults, config.App.Flags.Force); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("%s: file saved %q\n", config.App.Name, p)

	return nil
}

// editConfig edits the config file.
func editConfig(p string) error {
	if !files.Exists(p) {
		return files.ErrFileNotFound
	}

	te, err := editor.New(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := te.EditFile(p); err != nil {
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
		return nil, fmt.Errorf("%w", err)
	}

	if cfg == nil {
		return nil, fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return cfg, nil
}

// loadConfig loads the menu configuration YAML file.
func loadConfig(p string) error {
	cfg, err := getConfig(p)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return fmt.Errorf("%w", err)
	}

	if cfg == nil {
		slog.Debug("configfile is empty or not found. loading defaults")
		return nil
	}

	config.Fzf = cfg.Menu

	return nil
}

// printConfigJSON prints the config file as JSON.
func printConfigJSON(p string) error {
	cfg, err := getConfig(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	j, err := port.ToJSON(cfg)
	if err != nil {
		return fmt.Errorf("%w", err)
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
