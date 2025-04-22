package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	// createConfFlag is the flag for creating a new config file.
	createConfFlag bool

	// colorSchemeFlag list available color schemes.
	colorSchemeFlag bool
)

var ErrConfigFileNotFound = errors.New("config file not found")

// createAppConfig dumps the app configuration to a YAML file.
func createAppConfig(p string) error {
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Info(format.PaddedLine("Destination:", color.Text(p).Italic()) + "\n").Row("\n")
	if !terminal.Confirm(f.Question("continue?").String(), "y") {
		return nil
	}

	if err := files.YamlWrite(p, config.Defaults); err != nil {
		return fmt.Errorf("%w", err)
	}

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
	if err != nil {
		if !errors.Is(err, ErrConfigFileNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if cfg == nil {
		slog.Warn("configfile is empty. loading defaults")
		return nil
	}

	config.Fzf = cfg.Menu
	config.App.Colorscheme = cfg.Colorscheme

	return loadColorSchemes()
}

// printConfigJSON prints the config file as JSON.
func printConfigJSON(p string) error {
	cfg, err := getConfig(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(string(format.ToJSON(cfg)))

	return nil
}

// ExportColorScheme saves given colorscheme to a YAML file in the colorschemes
// path.
func ExportColorScheme(cs *color.Scheme) error {
	// TODO: use it or lose it
	p := config.App.Path.Colorschemes
	if p == "" {
		return fmt.Errorf("%w for colorschemes", files.ErrPathNotFound)
	}
	slog.Info("colorscheme path", "path", p)

	fn := filepath.Join(p, cs.Name+".yaml")
	if err := files.YamlWrite(fn, cs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// loadColorSchemes loads available colorschemes.
func loadColorSchemes() error {
	fs, err := files.FindByExtList(config.App.Path.Colorschemes, "yaml")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	for _, s := range fs {
		var cs *color.Scheme
		if err := files.YamlRead(s, &cs); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := cs.Validate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		color.Schemes[cs.Name] = cs
	}

	return nil
}

// printColorSchemes prints a list of available colorschemes.
func printColorSchemes() error {
	if !files.Exists(config.App.Path.Colorschemes) {
		return fmt.Errorf("%w for colorschemes", files.ErrPathNotFound)
	}

	if err := loadColorSchemes(); err != nil {
		return err
	}

	keys := make([]string, 0, len(color.Schemes))
	for k := range color.Schemes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	f := frame.New(frame.WithColorBorder(color.Gray))
	h := color.BrightYellow("ColorSchemes " + strconv.Itoa(len(keys)) + " found\n").String()
	f.Header(h).Row("\n")
	for _, k := range keys {
		cs, _ := color.Schemes[k]
		c := strconv.Itoa(cs.Palette.Len())
		f.Mid(fmt.Sprintf("%-*s %v\n", 20, cs.Name, color.Gray(" ("+c+" colors)")))
	}
	f.Flush()

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
			return createAppConfig(fn)
		case Edit:
			return editConfig(fn)
		case colorSchemeFlag:
			return printColorSchemes()
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
	f.BoolVarP(&colorSchemeFlag, "schemes", "s", false, "list available color schemes")
	f.BoolVarP(&Edit, "edit", "e", false, "edit config")
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	// set and hide persistent flag
	f.StringVar(&WithColor, "color", "always", "")
	f.StringVarP(&DBName, "name", "n", "", "database name")
	_ = configCmd.PersistentFlags().MarkHidden("help")
	_ = f.MarkHidden("name")
	_ = f.MarkHidden("color")
	_ = f.MarkHidden("help")
	rootCmd.AddCommand(configCmd)
}
