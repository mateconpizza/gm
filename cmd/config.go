package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/sys/files"
)

// dumpConf is the flag for dumping the config.
var dumpConf bool

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
		log.Println("menu configfile not found. loading defaults")
		return nil
	}
	var menuConfig *config.ConfigFile
	if err := files.ReadYamlFile(p, &menuConfig); err != nil {
		return fmt.Errorf("%w", err)
	}

	if menuConfig == nil {
		log.Println("menu configfile is empty. loading defaults")
		return nil
	}

	if err := menu.ValidateConfig(menuConfig.Menu); err != nil {
		return fmt.Errorf("%w", err)
	}

	config.Fzf = menuConfig.Menu

	return nil
}

// configCmd configuration management.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	RunE: func(cmd *cobra.Command, _ []string) error {
		fn := config.App.Path.ConfigFile
		switch {
		case dumpConf:
			return dumpAppConfig(fn)
		case Edit:
			return editConfig(fn)
		}

		return cmd.Usage()
	},
}

func init() {
	f := configCmd.Flags()
	f.BoolP("help", "h", false, "Hidden help")
	f.BoolVarP(&dumpConf, "dump", "d", false, "dump config")
	f.BoolVarP(&Edit, "edit", "e", false, "edit config")
	// set and hide persistent flag
	f.StringVar(&WithColor, "color", "always", "")
	f.StringVarP(&DBName, "name", "n", "", "database name")
	_ = configCmd.PersistentFlags().MarkHidden("help")
	_ = f.MarkHidden("name")
	_ = f.MarkHidden("color")
	rootCmd.AddCommand(configCmd)
}
