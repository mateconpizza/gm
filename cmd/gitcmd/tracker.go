package gitcmd

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/files"
)

func newTrackerCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tracker",
		Short:   "configure repository tracking",
		Aliases: []string{"t", "track"},
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

			return status(d.Console(), app, reposStr)
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

			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			c := ui.NewDefaultConsole(ctx, func(err error) { sys.ErrAndExit(err) })

			return gitops.TrackManager(ctx, m, c, dbFiles)
		},
	}

	return c
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
