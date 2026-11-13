package gitops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/session"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/git"
)

type Menu interface {
	Select(ctx context.Context, items []*git.Repo) ([]*git.Repo, error)
}

type Terminal interface {
	Choose(ctx context.Context, q string, opts []string, def string) (string, error)
}

type (
	// GitPuller handles remote repository data ingestion.
	GitPuller struct {
		srcDir string      // Base directory where cloned assets are held
		dstDir string      // Output directory path for generated databases
		found  []string    // Discovered directory names matching the lookup criteria
		repos  []*git.Repo // Parsed structural instances ready for ingestion

		console *ui.Console
		writer  io.Writer
		logger  *slog.Logger
	}

	PullerOpt func(*GitPuller)
)

func NewPuller(c *ui.Console, srcDir, dstDir string, opts ...PullerOpt) *GitPuller {
	gp := &GitPuller{
		srcDir:  srcDir,
		dstDir:  dstDir,
		console: c,
		writer:  os.Stdout,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(gp)
	}
	gp.logger = gp.logger.With("component", "git-puller")

	return gp
}

func WithPullerLogger(l *slog.Logger) PullerOpt { return func(gp *GitPuller) { gp.logger = l } }

// Repos returns the loaded repositories if processing has been initiated.
func (gp *GitPuller) Repos() []*git.Repo { return gp.repos }

// Pull pulls bookmarks from remote git repository and replicate the
// repositories as databases.
func (gp *GitPuller) Pull(sess *session.Session) error {
	log := gp.logger.With("fn", "pull", "src", gp.srcDir, "dst", gp.dstDir)
	log.Debug("starting pull")

	if err := gp.scan(); err != nil {
		return err
	}

	if err := gp.loadAll(sess); err != nil {
		return err
	}

	rc, err := resolveConfig(gp.srcDir)
	if err != nil {
		return err
	}

	printHeader(gp, rc)
	log.Debug("pull completed", "repos", len(gp.repos))

	return nil
}

// Read read bookmarks in the repository.
func (gp *GitPuller) Read(ctx context.Context, gr *git.Repo) error {
	log := gp.logger.With("fn", "read", "repo", gr.Name())

	_, err := gr.Stats()
	if err != nil {
		log.Debug("skipping repo, stats failed", "error", err)
		fmt.Fprintln(gp.writer, "skipping "+gr.Name(), err.Error())
		return nil
	}

	log.Debug("reading repo")

	return gr.Read(ctx)
}

func (gp *GitPuller) selection(ctx context.Context, m Menu, t Terminal) error {
	log := gp.logger.With("fn", "selection")

	opts := []string{"yes", "no"}
	n := len(gp.found)
	if n > 1 {
		opts = append(opts, "select")
	}

	opt, err := t.Choose(ctx, "import repositories?", opts, "n")
	if err != nil {
		return err
	}
	log.Debug("option chosen", "option", opt, "found", n)

	switch opt {
	case "yes":
		return nil
	case "no":
		return application.ErrActionAborted
	case "select":
		repos, err := m.Select(ctx, gp.repos)
		if err != nil {
			return err
		}

		log.Debug("repos selected", "before", len(gp.repos), "after", len(repos))
		gp.repos = repos
	}

	return nil
}

// printDetails outputs a repository's metadata profile and summary metrics.
func (gp *GitPuller) printDetails(gr *git.Repo) error {
	log := gp.logger.With("fn", "print-details", "repo", gr.Name())

	f, p := gp.console.Frame(), gp.console.Palette()
	f.Rowln().Info(p.Bold.Sprintf("Repository %q\n", gr.Name()))

	stats, err := gr.Stats()
	if err != nil {
		return err
	}

	if stats.Bookmarks == 0 {
		log.Debug("repo has no bookmarks, skipping")
		skip := p.BrightYellow.Wrap("skipping ", p.Italic)
		gp.console.Warning(skip + gr.Name() + ": no bookmark found").Ln().Flush()
		return git.ErrGitRepoEmpty
	}

	log.Debug("printing stats", "bookmarks", stats.Bookmarks)
	printStats(gp.console, stats)

	return nil
}

// scan finds matching subdirectories in the working root.
func (gp *GitPuller) scan() error {
	log := gp.logger.With("fn", "scan", "src", gp.srcDir)

	repos, err := files.ListRootFolders(gp.srcDir, ".git")
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		log.Debug("no git repositories found")
		return git.ErrGitNoRepos
	}

	log.Debug("repositories found", "count", len(repos), "repos", repos)
	gp.found = repos

	return nil
}

// loadAll discovers and loads metadata for all available repositories.
func (gp *GitPuller) loadAll(sess *session.Session) error {
	log := gp.logger.With("fn", "load-all")
	ops := NewRepoOps(sess)

	for _, repoName := range gp.found {
		fullpath := filepath.Join(gp.srcDir, repoName)
		log.Debug("loading repo", "repo", repoName, "path", fullpath)

		gr := git.NewRepo(
			repoName,
			fullpath,
			ops.Reader(),
		)

		gp.repos = append(gp.repos, gr)
	}

	log.Debug("repos loaded", "count", len(gp.repos))

	return nil
}

// printStats writes repository metadata to the user interface.
func printStats(c *ui.Console, stats *git.RepoStats) {
	f, p := c.Frame(), c.Palette()
	f.Midln(txt.PaddedLine(p.BrightYellow.Sprint("records:"), stats.Bookmarks)).
		Midln(txt.PaddedLine(p.BrightBlue.Sprint("tags:"), stats.Tags))

	if stats.Favorites > 0 {
		f.Midln(txt.PaddedLine(p.BrightRed.Sprint("favorites:"), stats.Favorites))
	}
	if stats.Archived > 0 {
		f.Midln(txt.PaddedLine(p.BrightRed.Sprint("wayback:"), stats.Archived))
	}
	if stats.TotalVisits > 0 {
		f.Midln(txt.PaddedLine("visits:", stats.TotalVisits))
	}

	f.Flush()
}

// printHeader writes the operational overview to the terminal.
func printHeader(rp *GitPuller, rc RepoConfig) {
	var (
		c      = rp.console
		p      = c.Palette()
		f      = c.Frame()
		y      = func(s string) string { return p.BrightYellow.Wrap(s, p.Bold) }
		square = func() string { return y(c.Glyphs().Square + " ") }
	)

	path := files.CollapseHomeDir(rp.dstDir)

	fmt.Fprintln(rp.console.Writer())

	rp.console.NewBannerBuilder().
		WithTitle("Repository cloned successfully").
		WithSubtitle(txt.PaddedLine("destination:", path)).
		Build().
		Midln(txt.PaddedLine(p.Dim.Sprint("encryption:"), p.Blue.Sprint(rc.Encryption))).
		Rowln().
		CustomFunc(square, p.Bold.Sprint("Found repositories")).
		Ln().
		Flush()

	t := p.BrightCyan.Sprint(RepoFormatJSON)
	if rc.Format == RepoFormatYAML {
		t = p.Orange.Sprint(RepoFormatYAML)
	}

	for _, gr := range rp.repos {
		stats, err := gr.Stats()
		if err != nil {
			continue
		}

		b := p.Dim.Sprintf(" (%d bookmarks)", stats.Bookmarks)
		f.Rowln(txt.PaddedLine(gr.Name(), t+b))
	}

	f.Rowln().Flush()
}
