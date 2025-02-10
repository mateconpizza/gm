package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/menu"
)

var menuConfig bool

// configCmd configuration management.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Usage()
	},
}

// configDumpCmd dump to yaml file.
var configDumpCmd = &cobra.Command{
	Use:     "dump",
	Short:   "Dump config to yaml file",
	Aliases: []string{"d"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if menuConfig {
			if err := menu.DumpConfig(Force); err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		return cmd.Usage()
	},
}

func init() {
	f := configCmd.Flags()
	f.BoolP("help", "h", false, "Hidden help")
	f.StringVarP(&DBName, "name", "n", "", "database name")
	_ = configCmd.PersistentFlags().MarkHidden("help")
	_ = f.MarkHidden("name")
	configDumpCmd.Flags().BoolVar(&menuConfig, "menu", false, "dump menu config")
	configCmd.AddCommand(configDumpCmd)
	rootCmd.AddCommand(configCmd)
}
