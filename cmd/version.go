/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"gomarks/pkg/constants"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("%s v%s\n", constants.AppName, constants.AppVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
