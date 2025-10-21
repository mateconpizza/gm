package git

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

type RepoProcessorOption func(*RepoProcessorOptions)

type RepoProcessorOptions struct {
	ctx context.Context
}

// RepoProcessor handles the processing of a single repository.
type RepoProcessor struct {
	Console  *ui.Console
	Root     string
	DestPath string
	Repos    []string
	result   *PullResult
	*RepoProcessorOptions
}

func WithRPContext(ctx context.Context) RepoProcessorOption {
	return func(o *RepoProcessorOptions) {
		o.ctx = ctx
	}
}

// PullResult tracks the results of a pull operation.
type PullResult struct {
	TotalBookmarks      int
	TotalReposProcessed int
	TotalSkipped        int
}

// processRepositories processes all repositories and returns aggregated results.
func (rp *RepoProcessor) processRepositories() error {
	for _, repoName := range rp.Repos {
		count, err := rp.processRepository(repoName)
		if err != nil {
			return err
		}

		if count > 0 {
			rp.result.TotalBookmarks += count
			rp.result.TotalReposProcessed++
		}
	}

	return nil
}

// processRepository processes a single repository and returns the number of bookmarks added.
func (rp *RepoProcessor) processRepository(repoName string) (int, error) {
	repoPath := filepath.Join(rp.Root, repoName)

	rp.Console.Frame().Rowln().Info(rp.Console.Palette().Bold(fmt.Sprintf("Repository %q\n", repoName)))

	// Read and display summary
	sum, err := rp.readSummary(repoPath)
	if err != nil {
		return 0, err
	}
	rp.displaySummary(sum)

	// Read bookmarks from git
	bookmarks, err := readBookmarks(rp.ctx, rp.Root, repoPath)
	if err != nil {
		return 0, err
	}

	// Insert into database
	return rp.insertBookmarks(repoName, bookmarks)
}

// readSummary reads the summary.json file from a repository.
func (rp *RepoProcessor) readSummary(repoPath string) (*SyncGitSummary, error) {
	sum := NewSummary()
	summaryPath := filepath.Join(repoPath, SummaryFileName)
	if err := files.JSONRead(summaryPath, sum); err != nil {
		return nil, fmt.Errorf("reading summary: %w", err)
	}

	return sum, nil
}

// displaySummary shows repository statistics.
func (rp *RepoProcessor) displaySummary(sum *SyncGitSummary) {
	f := rp.Console.Frame()
	f.Midln(txt.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(txt.PaddedLine(rp.Console.Palette().BrightBlue("tags:"), sum.RepoStats.Tags))

	if sum.RepoStats.Favorites > 0 {
		f.Midln(txt.PaddedLine(rp.Console.Palette().BrightRed("favorites:"), sum.RepoStats.Favorites))
	}

	f.Flush()
}

// insertBookmarks inserts bookmarks into the local database.
func (rp *RepoProcessor) insertBookmarks(repoName string, bookmarks []*bookmark.Bookmark) (int, error) {
	// Open or create database
	r, err := rp.openDatabase(repoName)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	// Initialize database if needed
	if err := r.Init(rp.ctx); err != nil {
		return 0, fmt.Errorf("initializing database: %w", err)
	}

	// Deduplicate and insert
	deduped := port.Deduplicate(rp.ctx, rp.Console, r, bookmarks)
	if len(deduped) == 0 {
		rp.result.TotalSkipped += len(bookmarks)
		return 0, nil
	}

	rp.result.TotalSkipped += len(bookmarks) - len(deduped)
	if err := r.InsertMany(rp.ctx, deduped); err != nil {
		return 0, err
	}

	return len(deduped), nil
}

// openDatabase opens an existing database or creates a new one.
func (rp *RepoProcessor) openDatabase(repoName string) (*db.SQLite, error) {
	dbPath := files.EnsureSuffix(filepath.Join(rp.DestPath, repoName), ".db")

	if files.Exists(dbPath) {
		return db.New(dbPath)
	}

	return db.Init(dbPath)
}

// displaySummary shows the final summary of the pull operation.
func (rp *RepoProcessor) displayPullSummary() {
	f, p := rp.Console.Frame(), rp.Console.Palette()
	r := rp.result
	pad := txt.PaddedLine

	f.Ln().Headerln(p.Bold("Summary:")).
		Midln(pad("Repos:", fmt.Sprintf("%d found", len(rp.Repos))))

	if r.TotalReposProcessed > 0 {
		f.Midln(pad(p.BrightRed("Processed:"), r.TotalReposProcessed))
	}
	if r.TotalSkipped > 0 {
		f.Midln(pad(p.BrightYellow("Skipped:"), r.TotalSkipped))
	}

	message := fmt.Sprintf("%d bookmarks", r.TotalBookmarks)
	if r.TotalBookmarks == 0 {
		message = "No bookmark added"
	}
	f.Midln(pad(p.BrightBlue("Added:"), message))

	f.Flush()
}

// searchRepositories searches for repos in the root folder.
func (rp *RepoProcessor) searchRepositories() error {
	repos, err := files.ListRootFolders(rp.Root, ".git")
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return ErrGitNoRepos
	}

	rp.Repos = repos

	return nil
}

// Pull pulls bookmarks from remote git repository and replicate the
// repositories as databases.
func (rp *RepoProcessor) Pull() error {
	if err := rp.searchRepositories(); err != nil {
		return err
	}

	if err := rp.processRepositories(); err != nil {
		return err
	}

	rp.displayPullSummary()

	return nil
}

func NewRepoProcessor(c *ui.Console, g *Manager, a *config.Config, opts ...RepoProcessorOption) *RepoProcessor {
	o := &RepoProcessorOptions{}
	for _, fn := range opts {
		fn(o)
	}

	if o.ctx == nil {
		o.ctx = context.Background()
	}

	return &RepoProcessor{
		Console:              c,
		Root:                 g.RepoPath,
		DestPath:             a.Path.Data,
		result:               &PullResult{},
		RepoProcessorOptions: o,
	}
}
