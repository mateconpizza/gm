// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"fmt"

	"gomarks/pkg/app"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("%s v%s\n", app.Config.Name, app.Config.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
