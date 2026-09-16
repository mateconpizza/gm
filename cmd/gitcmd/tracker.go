package gitcmd

import (
	"fmt"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tracker",
		Short:   "configure repository tracking",
		Aliases: []string{"t", "track"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c := ui.NewDefaultConsole(app.Flags.Color, func(err error) {
				app.Exit(err)
			})
			return gitops.TrackMgrStatus(c, app)
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
			return cmdutil.Run(cmd, args, gitops.NewTrack)
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
			return cmdutil.Run(cmd, args, gitops.Untrack)
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
			dbFiles, err := files.Find(app.Path.Home(), "*.db")
			if err != nil {
				return fmt.Errorf("finding db files: %w", err)
			}

			gm, err := gitops.NewManager(&gitops.ManagerConfig{
				Root:    app.Path.Git(),
				Writer:  app.Git.Writer(),
				Version: app.Version(),
				Color:   app.Flags.Color,
			})
			if err != nil {
				return err
			}
			c := ui.NewDefaultConsole(app.Flags.Color, func(err error) {
				app.Exit(err)
			})
			return gitops.TrackMgr(cmd.Context(), gm, c, dbFiles)
		},
	}

	return c
}
