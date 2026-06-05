package gitops

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

type Menu interface {
	Select(items []*git.Repo) ([]*git.Repo, error)
}

type Terminal interface {
	Choose(context.Context, string, []string, string) (string, error)
}

// GitPuller handles remote repository data ingestion.
type GitPuller struct {
	srcDir string      // Base directory where cloned assets are held
	dstDir string      // Output directory path for generated databases
	found  []string    // Discovered directory names matching the lookup criteria
	repos  []*git.Repo // Parsed structural instances ready for ingestion

	console *ui.Console
}

// scan finds matching subdirectories in the working root.
func (gp *GitPuller) scan() error {
	repos, err := files.ListRootFolders(gp.srcDir, ".git")
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return git.ErrGitNoRepos
	}

	gp.found = repos

	return nil
}

// loadAll discovers and loads metadata for all available repositories.
func (gp *GitPuller) loadAll() error {
	for _, repoName := range gp.found {
		fullpath := filepath.Join(gp.srcDir, repoName)
		gr := git.NewRepo(repoName, fullpath, RepoFileReader())

		gp.repos = append(gp.repos, gr)
	}

	return nil
}

// Pull pulls bookmarks from remote git repository and replicate the
// repositories as databases.
func (gp *GitPuller) Pull() error {
	if err := gp.scan(); err != nil {
		return err
	}

	if err := gp.loadAll(); err != nil {
		return err
	}

	printHeader(gp)

	return nil
}

// Read read bookmarks in the repository.
func (gp *GitPuller) Read(ctx context.Context) error {
	for _, gr := range gp.repos {
		_, err := gr.Stats()
		if err != nil {
			fmt.Println("skipping "+gr.Name(), err.Error())
			continue
		}

		if err := gr.Read(ctx); err != nil {
			return err
		}
	}

	return nil
}

// PrintDetails outputs a repository's metadata profile and summary metrics.
func (gp *GitPuller) PrintDetails(gr *git.Repo) error {
	f, p := gp.console.Frame(), gp.console.Palette()
	f.Rowln().Info(p.Bold.Sprintf("Repository %q\n", gr.Name()))

	stats, err := gr.Stats()
	if err != nil {
		return err
	}

	if stats.Bookmarks == 0 {
		skip := p.BrightYellow.Wrap("skipping ", p.Italic)
		gp.console.Warning(skip + gr.Name() + ": no bookmark found").Ln().Flush()
		return git.ErrGitRepoEmpty
	}

	printStats(gp.console, stats)
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
func printHeader(rp *GitPuller) {
	var (
		p      = rp.console.Palette()
		f      = rp.console.Frame()
		y      = func(s string) string { return p.BrightYellow.Wrap(s, p.Bold) }
		square = func() string { return y(txt.GlyphSmallSquare.Prefix(" ")) }
	)

	path := files.CollapseHomeDir(rp.dstDir)

	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	f.Ln().
		CustomFunc(square, y("Repository cloned successfully")).Ln().
		Midln(p.Dim.Wrap("Path: "+path, p.Italic)).Rowln().
		CustomFunc(func() string {
			return txt.GlyphSmallSquare.Prefix(" ")
		}, p.Bold.Sprint("Found repositories")).
		Textln(comment)

	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if gpg.IsInitialized(rp.srcDir) {
		t = p.BrightMagenta.Wrap("GPG", p.Bold)
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

func (gp *GitPuller) Select(ctx context.Context, m Menu, t Terminal) error {
	opts := []string{"yes", "no"}
	n := len(gp.found)
	if n > 1 {
		opts = append(opts, "select")
	}

	opt, err := t.Choose(ctx, "import repositories?", opts, "n")
	if err != nil {
		return err
	}

	if n == 1 {
		return nil
	}

	switch opt {
	case "y", "yes":
		return nil
	case "n", "no":
		return sys.ErrActionAborted
	case "s", "select":
		repos, err := m.Select(gp.repos)
		if err != nil {
			return err
		}

		gp.repos = repos
	}

	return nil
}

// Repos returns the loaded repositories if processing has been initiated.
func (gp *GitPuller) Repos() []*git.Repo {
	return gp.repos
}

func NewPuller(c *ui.Console, srcDir, dstDir string) *GitPuller {
	return &GitPuller{
		srcDir:  srcDir,
		dstDir:  dstDir,
		console: c,
	}
}
