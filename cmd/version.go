package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
)

var versionCmd = &cobra.Command{
	Use:    "version",
	Short:  "print version information",
	Hidden: false,
	Run: func(_ *cobra.Command, args []string) {
		fmt.Println(app.PrettyVersion(Prettify))
	},
}

func init() {
	h := "print data in pretty format"
	versionCmd.PersistentFlags().BoolVarP(&Prettify, "pretty", "p", false, h)
	rootCmd.AddCommand(versionCmd)
}
