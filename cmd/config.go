package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/sys/files"
)

var (
	// dumpConfFlag is the flag for dumping the config.
	dumpConfFlag bool

	// colorSchemeFlag list available color schemes.
	colorSchemeFlag bool
)

// dumpAppConfig dumps the app configuration to a YAML file.
func dumpAppConfig(p string) error {
	if err := files.WriteYamlFile(p, config.Defaults); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// editConfig edits the config file.
func editConfig(p string) error {
	te, err := files.GetEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := te.EditFile(p); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// loadConfig loads the menu configuration YAML file.
func loadConfig(p string) error {
	if !files.Exists(p) {
		log.Println("configfile not found. loading defaults")
		return nil
	}

	var cfg *config.ConfigFile
	if err := files.ReadYamlFile(p, &cfg); err != nil {
		return fmt.Errorf("%w", err)
	}

	if cfg == nil {
		log.Println("configfile is empty. loading defaults")
		return nil
	}

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("%w", err)
	}

	config.Fzf = cfg.Menu
	config.App.Colorscheme.Name = cfg.Colorscheme

	return nil
}

// exportColorScheme saves given colorscheme to a YAML file in the colorschemes
// path.
func exportColorScheme(cs *color.Scheme) error {
	p := config.App.Colorscheme.Path
	if p == "" {
		return fmt.Errorf("%w for colorschemes", files.ErrPathNotFound)
	}
	log.Printf("colorscheme path: '%s'", p)

	fn := filepath.Join(p, cs.Name+".yaml")
	if err := files.WriteYamlFile(fn, cs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// printColorSchemes prints a list of available colorschemes.
func printColorSchemes() error {
	fs, err := files.FindByExtList(config.App.Colorscheme.Path, "yaml")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	for _, s := range fs {
		var cs *color.Scheme
		if err := files.ReadYamlFile(s, &cs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := cs.Validate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		color.Schemes[cs.Name] = cs
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
		case dumpConfFlag:
			return dumpAppConfig(fn)
		case Edit:
			return editConfig(fn)
		case colorSchemeFlag:
			return printColorSchemes()
		}

		return cmd.Usage()
	},
}

func init() {
	f := configCmd.Flags()
	f.BoolP("help", "h", false, "Hidden help")
	f.BoolVarP(&dumpConfFlag, "dump", "d", false, "dump config")
	f.BoolVarP(&colorSchemeFlag, "schemes", "s", false, "list available color schemes")
	f.BoolVarP(&Edit, "edit", "e", false, "edit config")
	// set and hide persistent flag
	f.StringVar(&WithColor, "color", "always", "")
	f.StringVarP(&DBName, "name", "n", "", "database name")
	_ = configCmd.PersistentFlags().MarkHidden("help")
	_ = f.MarkHidden("name")
	_ = f.MarkHidden("color")
	_ = f.MarkHidden("help")
	rootCmd.AddCommand(configCmd)
}
