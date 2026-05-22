package git

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

type Menu interface {
	Select(items []*RemoteRepo) ([]*RemoteRepo, error)
}

type PullerOptFun func(*PullerOptions)

type PullerOptions struct {
	ctx context.Context
}

// RemoteRepo represents a remote repository's structure discovered in the git
// payload.
type RemoteRepo struct {
	name      string
	fullpath  string
	bookmarks []*bookmark.Bookmark
	stats     *db.RepoStats
}

func (r *RemoteRepo) Load(ctx context.Context) error {
	// read and display summary
	sum, err := loadSummary(r.fullpath)
	if err != nil {
		return err
	}

	// read bookmarks from git
	bookmarks, err := readBookmarks(ctx, filepath.Base(r.fullpath), r.fullpath)
	if err != nil {
		return err
	}

	slices.SortFunc(bookmarks, func(a, b *bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	r.bookmarks = bookmarks
	r.stats = sum.RepoStats

	return nil
}

func (r *RemoteRepo) Name() string {
	return r.name
}

func (r *RemoteRepo) Bookmarks() []*bookmark.Bookmark {
	return r.bookmarks
}

func (r *RemoteRepo) String() string {
	return txt.PaddedLine(r.name, fmt.Sprintf("(bookmarks: %d)", r.stats.Bookmarks))
}

// newRemoteRepo creates a tracked remote repository instance.
func newRemoteRepo(name, fullpath string) *RemoteRepo {
	return &RemoteRepo{name: name, fullpath: fullpath}
}

// Puller handles remote repository data ingestion.
type Puller struct {
	srcDir string        // Base directory where cloned assets are held
	dstDir string        // Output directory path for generated databases
	found  []string      // Discovered directory names matching the lookup criteria
	items  []*RemoteRepo // Parsed structural instances ready for ingestion

	*PullerOptions
	console *ui.Console
}

func WithPullerContext(ctx context.Context) PullerOptFun {
	return func(o *PullerOptions) {
		o.ctx = ctx
	}
}

// loadAll discovers and loads metadata for all available repositories.
func (pu *Puller) loadAll() error {
	for _, repoName := range pu.found {
		fullpath := filepath.Join(pu.srcDir, repoName)

		repo := newRemoteRepo(repoName, fullpath)
		if err := repo.Load(pu.ctx); err != nil {
			return err
		}

		pu.items = append(pu.items, repo)
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
func (pu *Puller) Repos() []*RemoteRepo {
	return pu.items
}

// PrintDetails outputs a repository's metadata profile and summary metrics.
func (pu *Puller) PrintDetails(repo *RemoteRepo) error {
	f, p := pu.console.Frame(), pu.console.Palette()
	f.Rowln().Info(p.Bold.Sprintf("Repository %q\n", repo.Name()))

	if len(repo.bookmarks) == 0 {
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

	printHeader(pu)

	return pu.loadAll()
}

func NewRepoProcessor(c *ui.Console, srcDir, dstDir string, opts ...PullerOptFun) *Puller {
	o := &PullerOptions{}
	for _, fn := range opts {
		fn(o)
	}

	if o.ctx == nil {
		o.ctx = context.Background()
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

	f.Ln().
		CustomFunc(square, y("Repository cloned successfully")).Ln().
		Midln(p.Gray.Wrap("Path: "+path, p.Italic)).Rowln().
		CustomFunc(func() string {
			return txt.GlyphSmallSquare.Prefix(" ")
		}, p.Bold.Sprint("Found repositories")).Ln()

	for _, repo := range rp.found {
		f.Rowln(repo)
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
