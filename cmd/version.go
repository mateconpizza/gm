package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
)

// prettyVersion formats version in a pretty way.
func prettyVersion(morePretty bool) string {
	name := color.BrightBlue(config.App.Name).Bold().String()
	if morePretty {
		name = color.BrightBlue(config.App.Banner).String()
	}

	return fmt.Sprintf("%s v%s %s/%s", name, config.App.Version, runtime.GOOS, runtime.GOARCH)
}

var versionCmd = &cobra.Command{
	Use:    "version",
	Short:  "print version information",
	Hidden: false,
	Run: func(_ *cobra.Command, args []string) {
		fmt.Println(prettyVersion(Prettify))
	},
}

func init() {
	h := "print data in pretty format"
	versionCmd.PersistentFlags().BoolVarP(&Prettify, "pretty", "p", false, h)
	rootCmd.AddCommand(versionCmd)
}
