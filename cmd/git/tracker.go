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
	"github.com/mateconpizza/gm/pkg/files"
)

func newTrackerCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "tracker",
		Short: "track database in git",
		RunE:  trackerFunc,
	}

	c.Flags().SortFlags = false
	c.Flags().BoolVarP(&cfg.Flags.List, "list", "l", false, "status tracked databases")
	c.Flags().BoolVarP(&cfg.Flags.Track, "track", "t", false, "track database in git")
	c.Flags().BoolVarP(&cfg.Flags.Untrack, "untrack", "u", false, "untrack database in git")

	return c
}

var gitTrackerCmd = &cobra.Command{
	Use:   "tracker",
	Short: "track database in git",
	RunE:  trackerFunc,
}

func trackerFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })

	switch {
	case cfg.Flags.List:
		return status(c, cfg, gr.Tracker.Repos)
	case cfg.Flags.Track:
		terminal.NonInteractiveMode(true) // don't ask confirmation
		return track(c, gr)
	case cfg.Flags.Untrack:
		terminal.NonInteractiveMode(true)
		return untrack(c, gr)
	}

	return status(c, cfg, gr.Tracker.Repos)
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console, cfg *config.Config) error {
	dbFiles, err := files.Find(cfg.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.Frame().Rowln().Midln("Select which databases to track").Flush()

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

func status(c *ui.Console, cfg *config.Config, tracked []string) error {
	if len(tracked) == 0 {
		return nil
	}

	dbFiles, err := files.Find(cfg.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}
	p := c.Palette()
	c.Frame().Header("Databases tracked in " + p.Yellow.Wrap("git\n", p.Bold)).Rowln().Flush()

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

	p := c.Palette()
	q := p.Bold.Sprintf("Untrack %q?", gr.Loc.Name)
	if gr.Loc.DBName == config.MainDBName {
		q = p.Bold.Sprint("Untrack database \"" + "main\"")
	}
	if !c.Confirm(c.Warning(q).String(), "n") {
		return nil
	}

	if err := gr.Untrack("untracked"); err != nil {
		return err
	}

	fmt.Println(c.SuccessMesg(fmt.Sprintf("database %q untracked", gr.Loc.DBName)))

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

	fmt.Println(c.SuccessMesg(fmt.Sprintf("database %q tracked", gr.Loc.DBName)))

	return nil
}
