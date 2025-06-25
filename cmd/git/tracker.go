// git tracker tracks and untracks a database in git.
package git

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
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

//nolint:wrapcheck //ignore
func trackerFunc(cmd *cobra.Command, args []string) error {
	g, err := handler.NewGit(config.App.Path.Git)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := g.Tracker.Load(); err != nil {
		return fmt.Errorf("loading tracker: %w", err)
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
	)

	gr := g.NewRepo(config.App.DBPath)
	g.Tracker.SetCurrent(gr)

	switch {
	case tkFlags.status:
		return trackedStatus(c, g)
	case tkFlags.mgt:
		return management(c, g)
	case tkFlags.track:
		if ok := g.Tracker.Contains(gr); ok {
			return git.ErrGitTracked
		}

		return handler.GitTrackExportCommit(c, g, "new tracking")
	case tkFlags.untrack:
		if ok := g.Tracker.Contains(gr); !ok {
			return git.ErrGitNotTracked
		}

		return untrackDropCommit(c, g)
	}

	return cmd.Help()
}

// initGPGRepo creates a GPG repo for a tracked database.
func initGPGRepo(c *ui.Console, gm *git.Manager) error {
	gr := gm.Tracker.Current()
	if files.Exists(gr.Path) {
		return nil
	}

	if err := port.GitExport(gm); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			fmt.Print(c.WarningMesg(fmt.Sprintf("skipping %q, no bookmarks found\n", gr.DBName)))
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	if err := handler.GitCommit(gm, "Initializing encrypted repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg("GPG repository initialized\n"))

	return nil
}

// initJSONRepo creates a JSON repo for a tracked database.
func initJSONRepo(c *ui.Console, gm *git.Manager) error {
	gr := gm.Tracker.Current()

	if err := port.GitExport(gm); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			fmt.Print(c.WarningMesg(fmt.Sprintf("skipping %q, no bookmarks found\n", gr.DBName)))
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	if err := handler.GitCommit(gm, "Initializing repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg("JSON repository initialized\n"))

	return nil
}

// initTracking initializes a tracked repo in the git repository.
func initTracking(c *ui.Console, gm *git.Manager) error {
	if gpg.IsInitialized(gm.RepoPath) {
		return initGPGRepo(c, gm)
	}

	return initJSONRepo(c, gm)
}

// untrackDropCommit removes a tracked repo from the git repository.
func untrackDropCommit(c *ui.Console, gm *git.Manager) error {
	gr := gm.Tracker.Current()
	if !gm.Tracker.Contains(gr) {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.DBName)
	}

	q := color.Text(fmt.Sprintf("Untrack %q?", gr.Name)).Bold()
	if !c.T.Confirm(c.Warning(q.String()).String(), "n") {
		return nil
	}

	c.ReplaceLine(c.Warning(fmt.Sprintf("Untracking %q", gr.Name)).String())

	if err := gm.Tracker.Untrack(gr).Save(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := dropRepo(gm, gr); err != nil {
		return err
	}

	if err := gm.AddAll(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gm.Commit(fmt.Sprintf("[%s] %s", gr.DBName, "Untrack database")); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database %q untracked\n", gr.DBName)))

	return nil
}

// dropRepo removes the repo from the git repo.
func dropRepo(gm *git.Manager, gr *git.GitRepository) error {
	slog.Debug("dropping repo", "dbPath", gr.DBPath)

	if !gm.IsInitialized() {
		return fmt.Errorf("%w: %q", git.ErrGitNotInitialized, gr.DBName)
	}

	if !files.Exists(gr.Path) {
		slog.Debug("repo does not exist", "path", gr.Path)
		return nil
	}

	if err := files.RemoveAll(gr.Path); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console, gm *git.Manager) ([]string, error) {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return nil, fmt.Errorf("finding db files: %w", err)
	}

	c.F.Midln("Select which databases to track").Rowln().Flush()

	tracked := make([]string, 0, len(dbFiles))

	var idx int
	for i, f := range dbFiles {
		if filepath.Base(f) == config.DefaultDBName {
			idx = i
			break
		}
	}
	if idx != 0 {
		dbFiles[0], dbFiles[idx] = dbFiles[idx], dbFiles[0]
	}

	for _, dbPath := range dbFiles {
		gr := gm.NewRepo(dbPath)
		if gm.Tracker.Contains(gr) {
			fmt.Print(c.Info(fmt.Sprintf("%q is already tracked\n", gr.Name)))
			continue
		}

		q := fmt.Sprintf("Track %q?", gr.Name)
		if gr.DBName == config.DefaultDBName {
			q = fmt.Sprintf("Track %q database?", "default")
		}

		if !c.Confirm(q, "n") {
			c.ReplaceLine(c.Warning(fmt.Sprintf("skipping %q", gr.Name)).String())
			continue
		}

		if err := gm.Tracker.Track(gr).Save(); err != nil {
			return nil, fmt.Errorf("tracking repo: %w", err)
		}

		tracked = append(tracked, gr.DBPath)

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking %q", gr.Name)).String())
	}

	return tracked, nil
}

// management updates the tracked databases in the git repository.
func management(c *ui.Console, gm *git.Manager) error {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Headerln("Tracked database management\n").Flush()

	for _, dbPath := range dbFiles {
		gr := gm.NewRepo(dbPath)
		gm.Tracker.SetCurrent(gr)

		if !gm.Tracker.Contains(gr) {
			if err := handler.GitTrackExportCommit(c, gm, "update tracking"); err != nil {
				return err
			}

			continue
		}

		if err := untrackDropCommit(c, gm); err != nil {
			return err
		}
	}

	return nil
}

//nolint:funlen //ignore
func trackedStatus(c *ui.Console, gm *git.Manager) error {
	if len(gm.Tracker.List) == 0 {
		return git.ErrGitNoRepos
	}

	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Header("Databases tracked in " + color.Orange("git\n").Italic().String()).Rowln().Flush()

	repos := make([]*git.GitRepository, 0, len(gm.Tracker.List))

	for _, dbPath := range dbFiles {
		gr := gm.NewRepo(dbPath)
		repos = append(repos, gr)
	}

	dimmer := color.Gray
	untracked := make([]*git.GitRepository, 0, len(repos))

	var sb strings.Builder
	for _, gr := range repos {
		sb.Reset()

		if !gm.Tracker.Contains(gr) {
			untracked = append(untracked, gr)
			continue
		}

		sum := git.NewSummary()
		if err := handler.GitRepoStats(gr.DBPath, sum); err != nil {
			return fmt.Errorf("%w", err)
		}

		st := sum.RepoStats

		var parts []string
		if st.Bookmarks > 0 {
			parts = append(parts, fmt.Sprintf("%d bookmarks", st.Bookmarks))
		}

		if st.Tags > 0 {
			parts = append(parts, fmt.Sprintf("%d tags", st.Tags))
		}

		if st.Favorites > 0 {
			parts = append(parts, fmt.Sprintf("%d favorites", st.Favorites))
		}

		if len(parts) == 0 {
			parts = append(parts, "no bookmarks")
		}

		var t string
		if gpg.IsInitialized(gm.RepoPath) {
			t = color.Cyan("gpg ").String()
		} else {
			t = color.Cyan("json ").String()
		}

		s := strings.TrimSpace(fmt.Sprintf("(%s)", strings.Join(parts, ", ")))
		sb.WriteString(txt.PaddedLine(gr.Name, t+dimmer(s).Italic().String()))

		c.Success(sb.String() + "\n").Flush()
	}

	for _, gr := range untracked {
		sb.Reset()
		sb.WriteString(txt.PaddedLine(gr.Name, dimmer("(not tracked)\n").Italic().String()))
		c.Error(sb.String()).Flush()
	}

	return nil
}
