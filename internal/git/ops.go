package git

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

const JSONFileExt = ".json"

var (
	cbc = func(s string) string { return color.BrightCyan(s).String() }
	cbm = func(s string) string { return color.BrightMagenta(s).String() }
	cgi = func(s string) string { return color.Gray(s).Italic().String() }
	cri = func(s string) string { return color.BrightRed(s).Italic().String() }
)

// writeRepoStats updates the repo stats.
func writeRepoStats(ctx context.Context, gr *Repository) error {
	var (
		summary     *SyncGitSummary
		summaryPath = filepath.Join(gr.Loc.Path, SummaryFileName)
	)

	if !files.Exists(summaryPath) {
		slog.Debug("creating new summary", "db", gr.Loc.DBPath)
		// Create new summary with only RepoStats
		summary = NewSummary()
		if err := repoStats(ctx, gr.Loc.DBPath, summary); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		slog.Debug("updating summary", "db", gr.Loc.DBPath)
		// Load existing summary
		summary = NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := repoStats(ctx, gr.Loc.DBPath, summary); err != nil {
			return fmt.Errorf("updating repo stats: %w", err)
		}
	}

	// Save updated or new summary
	if _, err := files.JSONWrite(summaryPath, summary, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return nil
}

// repoStats returns a new RepoStats.
func repoStats(ctx context.Context, dbPath string, summary *SyncGitSummary) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	summary.RepoStats = &dbtask.RepoStats{
		Name:      r.Name(),
		Bookmarks: r.Count(ctx, "bookmarks"),
		Tags:      r.Count(ctx, "tags"),
		Favorites: r.CountFavorites(ctx),
	}

	summary.GenChecksum()

	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(ctx context.Context, gr *Repository, actionMsg string) error {
	err := writeRepoStats(ctx, gr)
	if err != nil {
		return err
	}

	gm := gr.Git
	// check if any changes
	changed, _ := gm.hasChanges()
	if !changed {
		return nil
	}

	if err := gm.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := gm.status()
	if err != nil {
		status = ""
	}
	if status != "" {
		status = "(" + status + ")"
	}

	actionMsg = strings.ToLower(actionMsg)
	dbName := files.StripSuffixes(gr.Loc.DBName)
	if err := gm.Commit(fmt.Sprintf("[%s] %s %s", dbName, actionMsg, status)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// records gets all records from the database.
func records(ctx context.Context, dbPath string) ([]*bookmark.Bookmark, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	bs, err := r.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting bookmarks: %w", err)
	}

	return bs, nil
}

// parseGitRepo loads a git repo into a database.
func parseGitRepo(a *app.Context, root, repoName string) (string, error) {
	a.Console.Frame.Rowln().Info(fmt.Sprintf(color.Text("Repository %q\n").Bold().String(), repoName))
	repoPath := filepath.Join(root, repoName)

	// read summary.json
	sum := NewSummary()
	if err := files.JSONRead(filepath.Join(repoPath, SummaryFileName), sum); err != nil {
		return "", fmt.Errorf("reading summary: %w", err)
	}

	a.Console.Frame.Midln(txt.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(txt.PaddedLine("tags:", sum.RepoStats.Tags)).
		Midln(txt.PaddedLine("favorites:", sum.RepoStats.Favorites)).Flush()

	if err := a.Console.ConfirmErr("Continue?", "y"); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	var (
		opt     string
		err     error
		choices = []string{"merge", "drop", "create", "select", "ignore"}
	)

	dbPath := filepath.Join(a.Cfg.Path.Data, sum.RepoStats.Name)
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	if !a.Console.Confirm(fmt.Sprintf("Import into %q database?", gr.Loc.DBName), "y") {
		// FIX:
		// - Limit options to:
		// 		- Current database (flag `--name`)?
		// 		- New database
		// 		- on "no/cancel", abort all process?
		return "", nil
	}

	gr.Git.SetRepoPath(repoPath)

	if files.Exists(dbPath) {
		a.Console.Warning(fmt.Sprintf("Database %q already exists\n", gr.Loc.DBName)).Flush()
		opt, err = a.Console.Choose("What do you want to do?", choices, "m")
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
	} else {
		opt = "new"
		gr, err = NewRepo(dbPath)
		if err != nil {
			return "", err
		}
		gr.Git.SetRepoPath(repoPath)
	}

	resultPath, err := parseGitRepoOpt(a, opt, gr)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

// parseGitRepoOpt handles the options for parseGitRepository.
func parseGitRepoOpt(a *app.Context, opt string, gr *Repository) (string, error) {
	ctx, c := a.Ctx, a.Console

	switch strings.ToLower(opt) {
	case "new":
		return handleOptNew(ctx, c, gr)
	case "c", "create":
		return handleOptCreate(ctx, c, gr)
	case "d", "drop":
		return handleOptDrop(ctx, c, gr)
	case "m", "merge":
		return handleOptMerge(ctx, c, gr)
	case "s", "select":
		return handleOptSelect(ctx, c, gr)
	case "i", "ignore":
		return handleOptIgnore(ctx, c, gr)
	default:
		return gr.Loc.DBPath, nil
	}
}

// removeRepoFiles removes all files in a repository.
//
// Leaving only the root directory and the SummaryFileName (summary.json).
func removeRepoFiles(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if root == path || d.IsDir() || d.Name() == SummaryFileName {
			return nil
		}

		return files.RemoveFilepath(path)
	})
}

func untrackRemoveRepo(gr *Repository, mesg string) error {
	if !gr.IsTracked() {
		return ErrGitNotTracked
	}

	if err := gr.Tracker.Untrack(gr.Loc.Hash); err != nil {
		return err
	}

	if err := gr.Tracker.Save(); err != nil {
		return err
	}

	if err := files.RemoveAll(gr.Loc.Path); err != nil {
		return err
	}

	if err := gr.Git.AddAll(); err != nil {
		return err
	}

	return gr.Commit(mesg)
}

func trackRepo(gr *Repository) error {
	if gr.IsTracked() {
		return ErrGitTracked
	}

	if err := gr.Export(); err != nil {
		return err
	}

	if err := gr.Tracker.Track(gr.Loc.Hash); err != nil {
		return err
	}

	if err := gr.Tracker.Save(); err != nil {
		return err
	}

	return gr.Commit("new tracking")
}

func initGPG(c *ui.Console, gr *Repository, k *gpg.Fingerprint) error {
	if !k.IsTrusted() {
		return fmt.Errorf("%w: %s", gpg.ErrKeyNotTrusted, k.UserID)
	}

	if err := gpg.Init(gr.Git.RepoPath, AttributesFile, k); err != nil {
		return fmt.Errorf("gpg init: %w", err)
	}

	// add diff to git config
	for k, v := range gpg.GitDiffConf {
		if err := gr.Git.setConfigLocal(k, strings.Join(v, " ")); err != nil {
			return err
		}
	}

	if err := gr.Commit("GPG repo initialized"); err != nil {
		return err
	}

	if c != nil {
		s := fmt.Sprintf("GPG repo initialized with key %q\n", k.UserID)
		fmt.Print(c.SuccessMesg(s))
	}

	return nil
}

func dropRepo(gr *Repository, mesg string) error {
	if err := removeRepoFiles(gr.Loc.Path); err != nil {
		return err
	}

	if err := gr.RepoStatsWrite(); err != nil {
		return err
	}

	if err := gr.Git.AddAll(); err != nil {
		return err
	}

	return gr.Commit(mesg)
}

// selectAndInsert prompts the user to select records to import.
func selectAndInsert(ctx context.Context, c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := readBookmarks(ctx, filepath.Dir(dbPath), repoPath)
	if err != nil {
		return err
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithArgs("--cycle"),
		menu.WithSettings(config.New().Menu.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
	)

	records := make([]bookmark.Bookmark, 0, len(bookmarks))
	for _, b := range bookmarks {
		records = append(records, *b)
	}

	slices.SortFunc(records, func(a, b bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	m.SetItems(records)
	m.SetPreprocessor(txt.Oneline)

	selected, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := make([]*bookmark.Bookmark, 0, len(selected))
	for i := range selected {
		bs = append(bs, &selected[i])
	}

	debookmarks := port.Deduplicate(ctx, c, r, bs)
	if err := r.InsertMany(ctx, debookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(debookmarks)
	if n > 0 {
		c.Frame.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
	}

	return nil
}

func repoStatus(c *ui.Console, gr *Repository) string {
	var (
		sb strings.Builder
		t  string
	)

	if !gr.IsTracked() {
		sb.WriteString(txt.PaddedLine(gr.Loc.Name, cgi("(not tracked)\n")))
		return c.Error(sb.String()).StringReset()
	}

	if gr.IsEncrypted() {
		t = cbm("gpg ")
	} else {
		t = cbc("json ")
	}

	name := gr.Loc.Name
	if name == files.StripSuffixes(config.MainDBName) {
		name = "main"
	}

	s := strings.TrimSpace(fmt.Sprintf("(%s)", gr.String()))
	sb.WriteString(txt.PaddedLine(name, t+cgi(s)))

	c.Success(sb.String() + "\n").Flush()

	return ""
}

func StatusRepo(c *ui.Console, dbPath string) (string, error) {
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	return repoStatus(c, gr), nil
}

// Info returns a prettify info of the repository.
func Info(c *ui.Console, dbPath string, cfg *config.Git) (string, error) {
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	if !gr.IsTracked() {
		return c.Frame.StringReset(), err
	}

	if !cfg.Enabled {
		return "", nil
	}

	c.Frame.Reset().Headerln(cri("git:"))

	sum, err := gr.Summary()
	if err != nil {
		return c.Frame.StringReset(), err
	}

	// remote
	if sum.GitRemote != "" {
		c.Frame.Rowln(txt.PaddedLine("remote:", sum.GitRemote))
	}

	// repo type
	t := cbc("JSON")
	if cfg.GPG {
		t = cbm("GPG")
	}
	c.Frame.Rowln(txt.PaddedLine("type:", t))

	if sum.LastSync != "" {
		tt, err := time.Parse(time.RFC3339, sum.LastSync)
		if err != nil {
			return c.Frame.StringReset(), err
		}

		lastSync := sum.LastSync + cgi(" ("+txt.RelativeTime(tt.Format(txt.TimeLayout))+")")
		c.Frame.Rowln(txt.PaddedLine("last sync:", lastSync))
		c.Frame.Success(txt.PaddedLine("sync:", true)).Ln()
	} else {
		c.Frame.Error(txt.PaddedLine("sync:", false)).Ln()
	}

	return c.Frame.StringReset(), nil
}

func handleOptNew(ctx context.Context, c *ui.Console, gr *Repository) (string, error) {
	if err := intoDBFromGit(ctx, c, gr); err != nil {
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptCreate(ctx context.Context, c *ui.Console, gr *Repository) (string, error) {
	var dbName string
	for dbName == "" {
		dbName = files.EnsureSuffix(c.Prompt("Enter new name: "), ".db")
	}
	dbPath := filepath.Join(filepath.Dir(gr.Loc.DBPath), dbName)

	newGr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}
	newGr.Git.SetRepoPath(gr.Git.RepoPath)

	opt, err := c.Choose("What do you want to do?", []string{"select", "merge"}, "m")
	if err != nil {
		return "", err
	}

	r, err := db.Init(dbPath)
	if err != nil {
		return "", err
	}
	if err := r.Init(ctx); err != nil {
		return "", fmt.Errorf("initializing database: %w", err)
	}

	return parseGitRepoOpt(app.New(ctx, app.WithDB(r), app.WithConsole(c)), opt, newGr)
}

func handleOptDrop(ctx context.Context, c *ui.Console, gr *Repository) (string, error) {
	c.Warning("Dropping database\n").Flush()
	if err := dbtask.DropFromPath(ctx, gr.Loc.DBPath); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return handleOptMerge(ctx, c, gr)
}

func handleOptMerge(ctx context.Context, c *ui.Console, gr *Repository) (string, error) {
	c.Info("Merging database\n").Flush()
	if err := mergeAndInsert(ctx, c, gr); err != nil {
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptSelect(ctx context.Context, c *ui.Console, gr *Repository) (string, error) {
	if err := selectAndInsert(ctx, c, gr.Loc.DBPath, gr.Git.RepoPath); err != nil {
		if errors.Is(err, menu.ErrFzfActionAborted) {
			return "", nil
		}
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptIgnore(_ context.Context, c *ui.Console, gr *Repository) (string, error) {
	repoName := files.StripSuffixes(filepath.Base(gr.Loc.DBPath))
	c.ReplaceLine(c.Warning(fmt.Sprintf("%s repo %q", color.Yellow("skipping"), repoName)).StringReset())
	return "", nil
}

// intoDBFromGit loads a git repo into a database.
func intoDBFromGit(ctx context.Context, c *ui.Console, gr *Repository) error {
	bookmarks, err := readBookmarks(ctx, gr.Loc.Git, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	// FIX: replace with `repository.New`
	store, err := db.Init(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := store.Init(ctx); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	r, err := db.New(store.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := r.InsertMany(ctx, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	c.Frame.Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), gr.Loc.DBName)).Flush()

	return nil
}

// mergeAndInsert merges non-duplicates records into database.
func mergeAndInsert(ctx context.Context, c *ui.Console, gr *Repository) error {
	r, err := db.New(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := readBookmarks(ctx, gr.Loc.Git, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = port.Deduplicate(ctx, c, r, bookmarks)
	if err := r.InsertMany(ctx, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := gr.Export(); err != nil {
		return err
	}
	if err := gr.Commit("imported bookmarks from git"); err != nil {
		return err
	}

	n := len(bookmarks)
	if n > 0 {
		c.Frame.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, gr.Loc.DBName)).Flush()
	}

	return nil
}
