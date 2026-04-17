// Package cli provides utilities for building and managing Cobra commands.
package cli

import (
	"fmt"
	"runtime"

	"github.com/mateconpizza/gm/pkg/ansi"
)

var (
	// SkipDBCheckAnnotation is used in subcmds declarations to skip the database
	// existence check.
	SkipDBCheckAnnotation = map[string]string{"skip-db-check": "true"}

	// databaseChecked tracks whether the database check has already been
	// performed in the current process.
	databaseChecked bool = false
)

// PrettyVersion formats version in a pretty way.
func PrettyVersion(appName, version string) string {
	return fmt.Sprintf(
		"%s v%s %s/%s",
		ansi.BrightBlue.Wrap(appName, ansi.Bold),
		version,
		runtime.GOOS,
		runtime.GOARCH,
	)
}
