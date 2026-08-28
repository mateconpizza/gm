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
			return gitops.TrackMgrStatus(ui.DefaultConsole, app)
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

			gm, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			return gitops.TrackMgr(cmd.Context(), gm, ui.DefaultConsole, dbFiles)
		},
	}

	return c
}
