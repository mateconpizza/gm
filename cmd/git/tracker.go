// Package git...
package git

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/files"
)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "tracker",
		Short: "track database with git",
		RunE: func(cmd *cobra.Command, args []string) error {
			gr, err := git.NewRepo(app.Path.Database)
			if err != nil {
				return err
			}

			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })

			switch {
			case app.Flags.List:
				return status(c, app, gr.Tracker.Repos)
			case app.Flags.Track:
				terminal.NonInteractiveMode(true) // don't ask confirmation
				return track(c, gr)
			case app.Flags.Untrack:
				terminal.NonInteractiveMode(true) // don't ask confirmation
				return untrack(c, gr)
			}

			return status(c, app, gr.Tracker.Repos)
		},
	}

	c.Flags().SortFlags = false
	c.Flags().BoolVarP(&app.Flags.List, "list", "l", false, "status tracked databases")
	c.Flags().BoolVarP(&app.Flags.Track, "track", "t", false, "track database in git")
	c.Flags().BoolVarP(&app.Flags.Untrack, "untrack", "u", false, "untrack database in git")

	return c
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console, app *application.App) error {
	dbFiles, err := files.Find(app.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.Frame().Rowln().Midln("Select which databases to track").Flush()

	files.PrioritizeFile(dbFiles, application.MainDBName)
	for i, dbPath := range dbFiles {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return fmt.Errorf("creating repo: %w", err)
		}

		if gr.IsTracked() {
			fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", gr.Loc.Name)))
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
			fmt.Fprintln(c.Writer())
		}
	}

	return nil
}

func status(c *ui.Console, app *application.App, tracked []string) error {
	if len(tracked) == 0 {
		return nil
	}

	dbFiles, err := files.Find(app.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	// move main database to the top
	files.PrioritizeFile(dbFiles, app.DBName)

	p := c.Palette()

	title := p.BrightYellow.With(p.Bold).
		Sprint("Git Tracked Databases")
	subtitle := p.Dim.With(p.Italic).
		Sprint("showing tracked databases with git")
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	c.Frame().
		CustomFunc(header, title).Ln().
		Headerln(subtitle).
		Rowln().
		Flush()

	var sb strings.Builder

	dbFiles = prioritizeTracked(dbFiles, tracked)
	for _, dbPath := range dbFiles {
		s, err := git.StatusRepo(c, dbPath)
		if err != nil {
			return err
		}

		if s == "" {
			continue
		}

		sb.WriteString(s)
	}

	fmt.Fprint(c.Writer(), sb.String())

	return nil
}

func untrack(c *ui.Console, gr *git.Repository) error {
	if !gr.IsTracked() {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.Loc.DBName)
	}

	p := c.Palette()
	q := p.Bold.Sprintf("Untrack %q?", gr.Loc.Name)
	if gr.Loc.DBName == application.MainDBName {
		q = p.Bold.Sprint("Untrack database \"" + "main\"")
	}
	if !c.Confirm(c.Warning(q).String(), "n") {
		return nil
	}

	if err := gr.Untrack(fmt.Sprintf("[%s] remove tracking", gr.Loc.Name)); err != nil {
		return err
	}

	fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("database %q untracked", gr.Loc.DBName)))

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

	fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("database %q tracked", gr.Loc.DBName)))

	return nil
}

func prioritizeTracked(dbFiles, tracked []string) []string {
	trackedSet := make(map[string]int, len(tracked))
	for i, name := range tracked {
		trackedSet[name] = i
	}

	priority := make([]string, 0, len(tracked))
	rest := make([]string, 0, len(dbFiles)-len(tracked))

	for _, path := range dbFiles {
		name := strings.TrimSuffix(filepath.Base(path), ".db")
		if _, ok := trackedSet[name]; ok {
			priority = append(priority, path)
		} else {
			rest = append(rest, path)
		}
	}

	// sort priority slice by tracked order
	slices.SortFunc(priority, func(a, b string) int {
		nameA := strings.TrimSuffix(filepath.Base(a), ".db")
		nameB := strings.TrimSuffix(filepath.Base(b), ".db")
		return cmp.Compare(trackedSet[nameA], trackedSet[nameB])
	})

	return append(priority, rest...)
}
