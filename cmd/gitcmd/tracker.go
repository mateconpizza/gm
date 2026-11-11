package gitcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tracker",
		Short:   "configure repository tracking",
		Aliases: []string{"t", "track"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.TrackMgrStatus(app, ui.NewDefaultConsole(app, app.Exit))
		},
	}

	c.AddCommand(
		newTrackCmd(app),
		newUntrackCmd(app),
		newMgrCmd(app),
	)

	return c
}

func newTrackCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "track",
		Short:   "track a database",
		Aliases: []string{"t", "add", "new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd.Context(), args, gitops.NewTrack)
		},
	}

	return c
}

func newUntrackCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "untrack",
		Short:   "untrack a database",
		Aliases: []string{"u", "remove", "rm", "r"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd.Context(), args, gitops.Untrack)
		},
	}

	return c
}

func newMgrCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "manager",
		Short:   "select which database to track",
		Aliases: []string{"mgr", "m"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
				return gitops.TrackMgr(ctx, app, app.Git.Manager(), d.Console(), app.Databases())
			})
		},
	}

	return c
}
