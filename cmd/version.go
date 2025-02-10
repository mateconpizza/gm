package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
)

// prettyVersion formats version in a pretty way.
func prettyVersion() string {
	name := color.BrightBlue(config.App.Name).Bold().String()
	return fmt.Sprintf("%s v%s %s/%s", name, config.App.Version, runtime.GOOS, runtime.GOARCH)
}

var versionCmd = &cobra.Command{
	Use:    "version",
	Short:  "Print version information",
	Hidden: false,
	Run: func(_ *cobra.Command, args []string) {
		fmt.Println(prettyVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
