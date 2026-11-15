package gitcmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/command"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tracker",
		Short:   "configure repository tracking",
		Aliases: []string{"t", "track"},
		Example: app.Example(`  $ {cmd} git tracker track --db {db}
  $ {cmd} git tracker new --db other
  $ {cmd} git tracker untrack
  $ {cmd} git tracker del --db {db}`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.TrackMgrStatus(app, ui.NewDefaultConsole(app, app.Exit))
		},
	}

	c.AddCommand(
		newTrackCmd(app),
		newUntrackCmd(app),
	)

	return c
}

func newTrackCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "track",
		Short:   "track a database",
		Aliases: []string{"add", "new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Run(cmd.Context(), args, gitops.NewTrack)
		},
	}

	return c
}

func newUntrackCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "untrack",
		Short:   "untrack a database",
		Aliases: []string{"remove", "rm", "r", "del"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Run(cmd.Context(), args, gitops.NewUntrack)
		},
	}

	return c
}
