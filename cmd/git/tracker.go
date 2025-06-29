// git tracker tracks and untracks a database in git.
package git

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
		return trackedStatus(c, gr.Tracker.List)
	case tkFlags.mgt:
		return management(c)
	case tkFlags.track:
		return handler.GitTrackExportCommit(c, gr, "new tracking")
	case tkFlags.untrack:
		return handler.GitUntrackDropCommit(c, gr)
	}

	return cmd.Help()
}

func initGPGRepo(c *ui.Console, gr *git.Repository) error {
	if files.Exists(gr.Loc.Path) {
		slog.Debug("repo already exists", "path", gr.Loc.Path)
		return nil
	}

	if err := gr.Export(); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			fmt.Print(c.WarningMesg(fmt.Sprintf("skipping %q, no bookmarks found\n", gr.Loc.DBName)))
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	if err := gr.Commit("initializing encrypted repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg("GPG repository initialized\n"))

	return nil
}

// initJSONRepo creates a JSON repo for a tracked database.
func initJSONRepo(c *ui.Console, gr *git.Repository) error {
	if err := gr.Export(); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			fmt.Print(c.WarningMesg(fmt.Sprintf("skipping %q, no bookmarks found\n", gr.Loc.DBName)))
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	if err := gr.Commit("initializing repo"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg("JSON repository initialized\n"))

	return nil
}

// initTracking initializes a tracked repo in the git repository.
func initTracking(c *ui.Console, gr *git.Repository) error {
	if gpg.IsInitialized(gr.Git.RepoPath) {
		return initGPGRepo(c, gr)
	}

	return initJSONRepo(c, gr)
}

// managementSelect select which database to track in the git repository.
func managementSelect(c *ui.Console) ([]string, error) {
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
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return nil, fmt.Errorf("creating repo: %w", err)
		}

		if gr.IsTracked() {
			fmt.Print(c.Info(fmt.Sprintf("%q is already tracked\n", gr.Loc.Name)))
			continue
		}

		q := fmt.Sprintf("Track %q?", gr.Loc.Name)
		if gr.Loc.DBName == config.DefaultDBName {
			q = fmt.Sprintf("Track %q database?", "default")
		}

		if !c.Confirm(q, "n") {
			c.ReplaceLine(c.Warning(fmt.Sprintf("skipping %q", gr.Loc.Name)).String())
			continue
		}

		if err := gr.Track(); err != nil {
			return nil, fmt.Errorf("tracking repo: %w", err)
		}

		tracked = append(tracked, gr.Loc.DBPath)

		c.ReplaceLine(c.Success(fmt.Sprintf("Tracking %q", gr.Loc.Name)).String())
	}

	return tracked, nil
}

// management updates the tracked databases in the git repository.
func management(c *ui.Console) error {
	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Headerln("Tracked database management\n").Flush()

	for _, dbPath := range dbFiles {
		newRepo, err := git.NewRepo(dbPath)
		if err != nil {
			return fmt.Errorf("creating repo: %w", err)
		}

		if !newRepo.IsTracked() {
			if err := handler.GitTrackExportCommit(c, newRepo, "new tracking"); err != nil {
				return err
			}
			continue
		}

		if err := handler.GitUntrackDropCommit(c, newRepo); err != nil {
			return err
		}
	}

	return nil
}

func trackedStatus(c *ui.Console, tracked []string) error {
	if len(tracked) == 0 {
		return nil
	}

	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}

	c.F.Header("Databases tracked in " + color.Orange("git\n").Italic().String()).Rowln().Flush()

	repos := make([]*git.Repository, 0, len(tracked))
	for _, dbPath := range dbFiles {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return fmt.Errorf("creating repo: %w", err)
		}

		repos = append(repos, gr)
	}

	dimmer := color.Gray
	untracked := make([]*git.Repository, 0, len(repos))

	var sb strings.Builder
	for _, gr := range repos {
		sb.Reset()
		if !gr.IsTracked() {
			untracked = append(untracked, gr)
			continue
		}

		var t string
		if gpg.IsInitialized(gr.Git.RepoPath) {
			t = color.Cyan("gpg ").String()
		} else {
			t = color.Cyan("json ").String()
		}

		s := strings.TrimSpace(fmt.Sprintf("(%s)", gr.String()))
		sb.WriteString(txt.PaddedLine(gr.Loc.Name, t+dimmer(s).Italic().String()))

		c.Success(sb.String() + "\n").Flush()
	}

	for _, gr := range untracked {
		sb.Reset()
		sb.WriteString(txt.PaddedLine(gr.Loc.Name, dimmer("(not tracked)\n").Italic().String()))
		c.Error(sb.String()).Flush()
	}

	return nil
}
