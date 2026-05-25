package git

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// TODO:
// - [ ] using `--yes` brakes the `import repositories?` question.
// - [ ] fix(git/reader): getting `fingerprintPath` on `gpgLoader` is horrible.

type Menu interface {
	Select(items []*Repo) ([]*Repo, error)
}

type PullerOptFun func(*PullerOptions)

type PullerOptions struct {
	IsGPG bool
}

// NewRemoteRepo creates a tracked remote repository instance.
func NewRemoteRepo(name, fullpath string) *Repo {
	return &Repo{
		name:     name,
		fullpath: fullpath,
	}
}

// Puller handles remote repository data ingestion.
type Puller struct {
	srcDir string   // Base directory where cloned assets are held
	dstDir string   // Output directory path for generated databases
	found  []string // Discovered directory names matching the lookup criteria
	items  []*Repo  // Parsed structural instances ready for ingestion

	*PullerOptions
	console *ui.Console
}

func WithPullerEncrypted(b bool) PullerOptFun {
	return func(o *PullerOptions) {
		o.IsGPG = b
	}
}

// loadAll discovers and loads metadata for all available repositories.
func (pu *Puller) loadAll() error {
	for _, repoName := range pu.found {
		fullpath := filepath.Join(pu.srcDir, repoName)

		repo := NewRemoteRepo(repoName, fullpath)
		if err := repo.LoadSummary(); err != nil {
			return err
		}

		pu.items = append(pu.items, repo)
	}

	return nil
}

// Read read bookmarks in the repository.
func (pu *Puller) Read(ctx context.Context) error {
	for _, repo := range pu.items {
		if err := repo.Read(ctx); err != nil {
			return err
		}
	}

	return nil
}

// loadSummary reads a repository summary file.
func loadSummary(repoPath string) (*SyncGitSummary, error) {
	sum := NewSummary()
	summaryPath := filepath.Join(repoPath, SummaryFileName)
	if err := files.JSONRead(summaryPath, sum); err != nil {
		return nil, fmt.Errorf("reading summary: %w", err)
	}

	return sum, nil
}

// printStats writes repository metadata to the user interface.
func printStats(c *ui.Console, stats *db.RepoStats) {
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

// scan finds matching subdirectories in the working root.
func (pu *Puller) scan() error {
	repos, err := files.ListRootFolders(pu.srcDir, ".git")
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return ErrGitNoRepos
	}

	pu.found = repos

	return nil
}

// Repos returns the loaded repositories if processing has been initiated.
func (pu *Puller) Repos() []*Repo {
	return pu.items
}

// PrintDetails outputs a repository's metadata profile and summary metrics.
func (pu *Puller) PrintDetails(repo *Repo) error {
	f, p := pu.console.Frame(), pu.console.Palette()
	f.Rowln().Info(p.Bold.Sprintf("Repository %q\n", repo.Name()))

	if repo.stats.Bookmarks == 0 {
		skip := p.BrightYellow.Wrap("skipping ", p.Italic)
		pu.console.Warning(skip + repo.Name() + ": no bookmark found").Ln().Flush()
		return ErrGitRepoEmpty
	}

	printStats(pu.console, repo.stats)
	return nil
}

// Pull pulls bookmarks from remote git repository and replicate the
// repositories as databases.
func (pu *Puller) Pull() error {
	if err := pu.scan(); err != nil {
		return err
	}

	if err := pu.loadAll(); err != nil {
		return err
	}

	printHeader(pu)

	return nil
}

func NewPuller(c *ui.Console, srcDir, dstDir string, opts ...PullerOptFun) *Puller {
	o := &PullerOptions{}
	for _, fn := range opts {
		fn(o)
	}

	return &Puller{
		console:       c,
		srcDir:        srcDir,
		dstDir:        dstDir,
		PullerOptions: o,
	}
}

// printHeader writes the operational overview to the terminal.
func printHeader(rp *Puller) {
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
		Midln(p.Gray.Wrap("Path: "+path, p.Italic)).Rowln().
		CustomFunc(func() string {
			return txt.GlyphSmallSquare.Prefix(" ")
		}, p.Bold.Sprint("Found repositories")).
		Textln(comment)

	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if rp.IsGPG {
		t = p.BrightMagenta.Wrap("GPG", p.Bold)
	}

	for _, repo := range rp.items {
		b := p.Dim.Sprintf(" (%d bookmarks)", repo.stats.Bookmarks)
		f.Rowln(txt.PaddedLine(repo.name, t+b))
	}

	f.Rowln().Flush()
}

func (pu *Puller) Select(m Menu) error {
	opts := []string{"yes", "no"}
	n := len(pu.found)
	if n > 1 {
		opts = append(opts, "select")
	}

	opt, err := pu.console.Choose("import repositories?", opts, "n")
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
		repos, err := m.Select(pu.items)
		if err != nil {
			return err
		}

		pu.items = repos
	}

	return nil
}
