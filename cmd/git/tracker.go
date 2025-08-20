// git tracker command
package git

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/files"
)

type trackerFlagsType struct {
	status  bool // pretty tracked databases status
	mgt     bool // repos management in git
	track   bool // track database in git
	untrack bool // untrack database in git
}

func init() {
	tfb := gitTrackerCmd.Flags().BoolVarP
	tfb(&tkFlags.track, "track", "t", false, "track database in git")
	tfb(&tkFlags.untrack, "untrack", "u", false, "untrack database in git")
	tfb(&tkFlags.status, "status", "s", false, "status tracked databases")
	tfb(&tkFlags.mgt, "manage", "m", false, "repos management in git")
}

var (
	tkFlags = trackerFlagsType{}

	gitTrackerCmd = &cobra.Command{
		Use:   "tracker",
		Short: "Track database in git",
		RunE:  trackerFunc,
	}
)

func trackerFunc(cmd *cobra.Command, _ []string) error {
	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
	)

	switch {
	case tkFlags.status:
		return status(c, gr.Tracker.List)
	case tkFlags.mgt:
		return management(c)
	case tkFlags.track:
		return track(c, gr)
	case tkFlags.untrack:
		return untrack(c, gr)
	}

	return cmd.Help()
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console) error {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Rowln().Midln("Select which databases to track").Rowln().Flush()

	files.PrioritizeFile(dbFiles, config.MainDBName)
	for i, dbPath := range dbFiles {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return fmt.Errorf("creating repo: %w", err)
		}

		if gr.IsTracked() {
			fmt.Print(c.Info(fmt.Sprintf("%q is already tracked\n", gr.Loc.Name)))
			continue
		}

		if !c.Confirm(fmt.Sprintf("Track %q?", gr.Loc.Name), "n") {
			c.ReplaceLine(c.Warning(fmt.Sprintf("skipping %q", gr.Loc.Name)).String())
			continue
		}

		if err := gr.Track(); err != nil {
			return fmt.Errorf("tracking repo: %w", err)
		}

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking %q", gr.Loc.Name)).String())
		if i != len(dbFiles)-1 {
			fmt.Println()
		}
	}

	return nil
}

// management updates the tracked databases in the git repository.
func management(c *ui.Console) error {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Headerln("Tracked database management").Rowln().Flush()
	files.PrioritizeFile(dbFiles, config.MainDBName)
	for i, dbPath := range dbFiles {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return fmt.Errorf("creating repo: %w", err)
		}

		if gr.IsTracked() {
			q := color.Text(fmt.Sprintf("Untrack %q?", gr.Loc.Name)).Bold()
			if gr.Loc.DBName == config.MainDBName {
				q = color.Text("Untrack database \"" + "main\"").Bold()
			}
			if !c.T.Confirm(c.Warning(q.String()).String(), "n") {
				c.ReplaceLine(c.Info(fmt.Sprintf("Unchange database %q", gr.Loc.Name)).String())
				continue
			}

			c.ReplaceLine(c.Warning(fmt.Sprintf("Untracking database %q", gr.Loc.Name)).String())

			if err := gr.Untrack("untracked"); err != nil {
				return err
			}

			fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.Loc.DBName)))
			if i != len(dbFiles)-1 {
				fmt.Println()
			}

			continue
		}

		if !c.Confirm(fmt.Sprintf("Track database %q?", gr.Loc.DBName), "n") {
			c.ReplaceLine(c.Info(fmt.Sprintf("Skipping database %q", gr.Loc.DBName)).String())

			continue
		}
		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking database %q", gr.Loc.DBName)).String())

		if err := gr.Track(); err != nil {
			return err
		}

		fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Loc.DBName)))
		if i != len(dbFiles)-1 {
			fmt.Println()
		}
	}

	return nil
}

func status(c *ui.Console, tracked []string) error {
	if len(tracked) == 0 {
		return nil
	}

	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Header("Databases tracked in " + color.Orange("git\n").Italic().String()).Rowln().Flush()
	files.PrioritizeFile(dbFiles, config.MainDBName)

	// move main database to the top
	for _, dbPath := range dbFiles {
		s, err := git.StatusRepo(c, dbPath)
		if err != nil {
			return err
		}
		fmt.Print(s)
	}

	return nil
}

func untrack(c *ui.Console, gr *git.Repository) error {
	if !gr.IsTracked() {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.Loc.DBName)
	}

	if !c.Confirm(fmt.Sprintf("Untrack database %q?", gr.Loc.DBName), "n") {
		return nil
	}

	if err := gr.Untrack("untracked"); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.Loc.DBName)))

	return nil
}

func track(c *ui.Console, gr *git.Repository) error {
	if gr.IsTracked() {
		return fmt.Errorf("%w: %q", git.ErrGitTracked, gr.Loc.DBName)
	}

	if !c.Confirm(fmt.Sprintf("Track database %q?", gr.Loc.DBName), "n") {
		return nil
	}

	if err := gr.Track(); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q tracked\n", gr.Loc.DBName)))

	return nil
}
