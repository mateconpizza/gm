package cmd

import (
	"fmt"

	"gomarks/pkg/color"
	"gomarks/pkg/constants"
)

func exampleUsage(l []string) string {
	var s string
	for _, line := range l {
		s += fmt.Sprintf("  %s %s", constants.AppName, line)
	}
	return s
}

func cmdTitle(s string) {
	fmt.Printf(
		"%s%s%s: %s, use %s%sctrl+c%s for quit\n\n",
		color.Bold,
		constants.AppName,
		color.Reset,
		s,
		color.Bold,
		color.Red,
		color.Reset,
	)
}
