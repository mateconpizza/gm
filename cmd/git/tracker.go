// Package git...
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

var gitTrackerCmd = &cobra.Command{
	Use:   "tracker",
	Short: "Track database in git",
	RunE:  trackerFunc,
}

func trackerFunc(cmd *cobra.Command, _ []string) error {
	app := config.New()
	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(
			terminal.WithContext(cmd.Context()),
			terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
		)),
	)

	switch {
	case app.Flags.List:
		return status(c, app, gr.Tracker.Repos)
	case app.Flags.Management:
		return management(c, app)
	case app.Flags.Track:
		terminal.NonInteractiveMode(true) // don't ask confirmation
		return track(c, gr)
	case app.Flags.Untrack:
		terminal.NonInteractiveMode(true)
		return untrack(c, gr)
	}

	return cmd.Help()
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console, app *config.Config) error {
	dbFiles, err := files.Find(app.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Rowln().Midln("Select which databases to track").Flush()

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
func management(c *ui.Console, app *config.Config) error {
	dbFiles, err := files.Find(app.Path.Data, "*.db")
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

func status(c *ui.Console, app *config.Config, tracked []string) error {
	if len(tracked) == 0 {
		return nil
	}

	dbFiles, err := files.Find(app.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Header("Databases tracked in " + color.Orange("git\n").Italic().String()).Rowln().Flush()

	// move main database to the top
	files.PrioritizeFile(dbFiles, config.MainDBName)

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
