package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

// createConfFlag is the flag for creating a new config file.
var createConfFlag bool

var ErrConfigFileNotFound = errors.New("config file not found")

// createConfig dumps the app configuration to a YAML file.
func createConfig(p string) error {
	if files.Exists(p) && !config.App.Force {
		f := color.BrightYellow("--force").Italic().String()
		return fmt.Errorf("%w. use %s to overwrite", files.ErrFileExists, f)
	}
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Info(txt.PaddedLine("File:", color.Text(p).Italic()) + "\n").Row("\n")
	if !terminal.Confirm(f.Question("create?").String(), "y") {
		return nil
	}

	if err := files.YamlWrite(p, config.Defaults, config.App.Force); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("%s: file saved %q\n", config.App.Name, p)

	return nil
}

// editConfig edits the config file.
func editConfig(p string) error {
	if !files.Exists(p) {
		return ErrConfigFileNotFound
	}
	te, err := files.NewEditor(config.App.Env.Editor)
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
		slog.Warn("configfile not found, loading defaults")
		return nil, ErrConfigFileNotFound
	}

	var cfg *config.ConfigFile
	if err := files.YamlRead(p, &cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if cfg == nil {
		return nil, ErrConfigFileNotFound
	}

	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return cfg, nil
}

// loadConfig loads the menu configuration YAML file.
func loadConfig(p string) error {
	cfg, err := getConfig(p)
	if err != nil && !errors.Is(err, ErrConfigFileNotFound) {
		return fmt.Errorf("%w", err)
	}

	if cfg == nil {
		slog.Warn("configfile is empty. loading defaults")
		return nil
	}

	config.Fzf = cfg.Menu
	config.App.Colorscheme = cfg.Colorscheme

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

// configCmd configuration management.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	RunE: func(cmd *cobra.Command, _ []string) error {
		fn := config.App.Path.ConfigFile
		switch {
		case createConfFlag:
			return createConfig(fn)
		case Edit:
			return editConfig(fn)
		case JSON:
			return printConfigJSON(fn)
		}

		return cmd.Usage()
	},
}

func init() {
	f := configCmd.Flags()
	f.BoolP("help", "h", false, "Hidden help")
	f.BoolVarP(&createConfFlag, "create", "c", false, "create config file")
	f.BoolVarP(&Edit, "edit", "e", false, "edit config")
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	// set and hide persistent flag
	f.StringVar(&WithColor, "color", "always", "")
	f.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	_ = configCmd.PersistentFlags().MarkHidden("help")
	_ = f.MarkHidden("name")
	_ = f.MarkHidden("color")
	_ = f.MarkHidden("help")
	rootCmd.AddCommand(configCmd)
}
