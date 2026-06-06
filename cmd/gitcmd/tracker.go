package gitcmd

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

// FIX:
// - [ ] after initializing a git repo, even when i dont track any database, it
// will create a `[repoName]/summary.json` (only on GPG repo)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tracker",
		Short:   "track database with git",
		Aliases: []string{"t"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			reposStr := m.Repos()

			if app.Flags.List {
				return status(d.Console(), app, reposStr)
			}

			return status(d.Console(), app, reposStr)
		},
	}

	c.Flags().SortFlags = false
	c.Flags().BoolVarP(&app.Flags.List, "list", "l", false, "status tracked databases")

	c.AddCommand(
		newTrackCmd(app),
		newUntrackCmd(app),
	)

	return c
}

// managementSelect select which database to track in the git repository.
func managementSelect(ctx context.Context, c *ui.Console, app *application.App, m *git.Mgr) error {
	dbFiles, err := files.Find(app.Path.Home(), "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.Frame().Rowln().
		Midln("Select which databases to track").
		Flush()

	files.PrioritizeFile(dbFiles, application.MainDBName)

	for i, dbPath := range dbFiles {
		name := filepath.Base(dbPath)
		if m.IsTracked(name) {
			fmt.Fprint(c.Writer(), c.Info(fmt.Sprintf("%q is already tracked\n", name)))
			continue
		}

		if !c.Confirm(ctx, fmt.Sprintf("Track %q?", name), "n") {
			continue
		}

		r, err := db.New(ctx, dbPath)
		if err != nil {
			return err
		}

		gr := gitops.NewRepo(m, r.Name(), git.WithRepoStore(r))
		if err := gitops.Track(ctx, r, m, gr); err != nil {
			return err
		}

		r.Close()

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking %q", name)).String())
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

	dbFiles, err := files.Find(app.Path.Home(), "*.db")
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

	m, err := gitops.NewManager(app)
	if err != nil {
		return err
	}

	var sb strings.Builder
	dbFiles = prioritizeTracked(dbFiles, tracked)
	for _, dbPath := range dbFiles {
		name := filepath.Base(dbPath)
		gr := gitops.NewRepo(m, name)

		s := gitops.TrackStatus(c, m, gr)
		if s == "" {
			continue
		}

		sb.WriteString(s)
	}

	fmt.Fprint(c.Writer(), sb.String())

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

func newTrackCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "track",
		Short:   "track a database",
		Aliases: []string{"t", "add", "new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return gitops.NewTrack(cmd.Context(), d)
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
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return gitops.Untrack(cmd.Context(), d)
		},
	}

	return c
}
