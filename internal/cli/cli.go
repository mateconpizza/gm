// Package cli provides utilities for building and managing Cobra commands.
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/ui/color"
)

var (
	// subCommands holds all registered CLI subcommands.
	subCommands []*cobra.Command

	// SkipDBCheckAnnotation is used in subcmds declarations to skip the database
	// existence check.
	SkipDBCheckAnnotation = map[string]string{"skip-db-check": "true"}

	// databaseChecked tracks whether the database check has already been
	// performed in the current process.
	databaseChecked bool = false
)

// Register appends one or more subcommands to the global registry.
func Register(cmd ...*cobra.Command) {
	subCommands = append(subCommands, cmd...)
}

// AttachTo attaches all registered subcommands to the given root command.
func AttachTo(cmd *cobra.Command) {
	cmd.AddCommand(subCommands...)
}

// PrettyVersion formats version in a pretty way.
func PrettyVersion(appName, version string) string {
	name := color.BrightBlue(appName).Bold().String()
	return fmt.Sprintf("%s v%s %s/%s", name, version, runtime.GOOS, runtime.GOARCH)
}
