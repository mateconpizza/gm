package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/sys/files"
)

// dumpConf is the flag for dumping the config.
var dumpConf bool

// editConfig edits the config file.
func editConfig() error {
	te, err := files.GetEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	p := config.App.Path.ConfigFile
	if err := te.EditFile(p); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// configCmd configuration management.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	RunE: func(cmd *cobra.Command, _ []string) error {
		switch {
		case dumpConf:
			return menu.DumpConfig()
		case Edit:
			return editConfig()
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
