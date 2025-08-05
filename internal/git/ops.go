package git

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/repository"
)

const JSONFileExt = ".json"

var (
	cbc = func(s string) string { return color.BrightCyan(s).String() }
	cbm = func(s string) string { return color.BrightMagenta(s).String() }
	cgi = func(s string) string { return color.Gray(s).Italic().String() }
	cri = func(s string) string { return color.BrightRed(s).Italic().String() }
)

// storeBookmarkAsJSON creates files structure.
//
//	root -> dbName -> domain
//
// Returns true if the file was created or updated, false if no changes were made.
func storeBookmarkAsJSON(rootPath string, b *bookmark.Bookmark, force bool) (bool, error) {
	domain, err := b.Domain()
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// domainPath: root -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// urlHash := domainPath -> urlHash.json
	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+JSONFileExt)

	updated, err := files.JSONWrite(filePathJSON, b.JSON(), force)
	if err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return updated, nil
}

// resolveFileConflictErr resolves a file conflict error.
// Returns true if the file was updated, false if no update was needed.
func resolveFileConflictErr(
	rootPath string,
	err error,
	filePathJSON string,
	b *bookmark.Bookmark,
) (bool, error) {
	if !errors.Is(err, files.ErrFileExists) {
		return false, err
	}

	bj := bookmark.BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// no need to update
	if bj.Checksum == b.Checksum {
		return false, nil
	}

	return storeBookmarkAsJSON(rootPath, b, true)
}

// writeRepoStats updates the repo stats.
func writeRepoStats(gr *Repository) error {
	var (
		summary     *SyncGitSummary
		summaryPath = filepath.Join(gr.Loc.Path, SummaryFileName)
	)

	if !files.Exists(summaryPath) {
		slog.Debug("creating new summary", "db", gr.Loc.DBPath)
		// Create new summary with only RepoStats
		summary = NewSummary()
		if err := repoStats(gr.Loc.DBPath, summary); err != nil {
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
		if err := repoStats(gr.Loc.DBPath, summary); err != nil {
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
func repoStats(dbPath string, summary *SyncGitSummary) error {
	r, err := repository.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	summary.RepoStats = &RepoStats{
		Name:      r.Name(),
		Bookmarks: r.Count("bookmarks"),
		Tags:      r.Count("tags"),
		Favorites: r.CountFavorites(),
	}

	summary.GenChecksum()

	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(gr *Repository, actionMsg string) error {
	err := writeRepoStats(gr)
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
	msg := fmt.Sprintf("[%s] %s %s", gr.Loc.DBName, actionMsg, status)
	if err := gm.Commit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// summaryString returns a string representation of the repo summary.
func summaryString(rs *RepoStats) string {
	var parts []string
	if rs.Bookmarks > 0 {
		parts = append(parts, fmt.Sprintf("%d bookmarks", rs.Bookmarks))
	}

	if rs.Tags > 0 {
		parts = append(parts, fmt.Sprintf("%d tags", rs.Tags))
	}

	if rs.Favorites > 0 {
		parts = append(parts, fmt.Sprintf("%d favorites", rs.Favorites))
	}

	if len(parts) == 0 {
		parts = append(parts, "no bookmarks")
	}

	return strings.Join(parts, ", ")
}

func summaryRead(gr *Repository) (*SyncGitSummary, error) {
	sum := NewSummary()
	if err := files.JSONRead(filepath.Join(gr.Loc.Path, SummaryFileName), sum); err != nil {
		return nil, fmt.Errorf("reading summary: %w", err)
	}

	return sum, nil
}

func summaryUpdate(gr *Repository) (*SyncGitSummary, error) {
	r, err := repository.New(gr.Loc.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	branch, err := gr.Git.branch()
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}

	remote, err := gr.Git.Remote()
	if err != nil {
		remote = ""
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	summary := &SyncGitSummary{
		GitBranch:          branch,
		GitRemote:          remote,
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "SHA-256",
		ClientInfo: &ClientInfo{
			Hostname:   hostname,
			Platform:   runtime.GOOS,
			Architect:  runtime.GOARCH,
			AppVersion: config.App.Info.Version,
		},
		RepoStats: &RepoStats{
			Name:      r.Name(),
			Bookmarks: r.Count("bookmarks"),
			Tags:      r.Count("tags"),
			Favorites: r.CountFavorites(),
		},
	}

	summary.GenChecksum()

	return summary, nil
}

// records gets all records from the database.
func records(dbPath string) ([]*bookmark.Bookmark, error) {
	r, err := repository.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	bs, err := r.All()
	if err != nil {
		return nil, fmt.Errorf("getting bookmarks: %w", err)
	}

	return bs, nil
}

// parseGitRepo loads a git repo into a database.
func parseGitRepo(c *ui.Console, root, repoName string) (string, error) {
	c.F.Rowln().Info(fmt.Sprintf(color.Text("Repository %q\n").Bold().String(), repoName))
	repoPath := filepath.Join(root, repoName)

	// read summary.json
	sum := NewSummary()
	if err := files.JSONRead(filepath.Join(repoPath, SummaryFileName), sum); err != nil {
		return "", fmt.Errorf("reading summary: %w", err)
	}

	c.F.Midln(txt.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(txt.PaddedLine("tags:", sum.RepoStats.Tags)).
		Midln(txt.PaddedLine("favorites:", sum.RepoStats.Favorites)).Flush()
	if err := c.ConfirmErr("Import records from this repo?", "y"); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	var (
		opt     string
		err     error
		choices = []string{"merge", "drop", "create", "select", "ignore"}
	)

	dbPath := filepath.Join(config.App.Path.Data, sum.RepoStats.Name)
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	if c.Confirm(fmt.Sprintf("Import into %q database?", gr.Loc.DBName), "y") {
		fmt.Printf("repoPath: %v\n", repoPath)
	}

	gr.Git.SetRepoPath(repoPath)

	if files.Exists(dbPath) {
		c.Warning(fmt.Sprintf("Database %q already exists\n", gr.Loc.DBName)).Flush()
		opt, err = c.Choose("What do you want to do?", choices, "m")
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

	resultPath, err := parseGitRepoOpt(c, opt, gr)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

// parseGitRepoOpt handles the options for parseGitRepository.
func parseGitRepoOpt(c *ui.Console, opt string, gr *Repository) (string, error) {
	switch strings.ToLower(opt) {
	case "new":
		return handleOptNew(c, gr)
	case "c", "create":
		return handleOptCreate(c, gr)
	case "d", "drop":
		return handleOptDrop(c, gr)
	case "m", "merge":
		return handleOptMerge(c, gr)
	case "s", "select":
		return handleOptSelect(c, gr)
	case "i", "ignore":
		return handleOptIgnore(c, gr)
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

	if err := gr.Tracker.Untrack(gr.Loc.Hash).Save(); err != nil {
		return err
	}

	if err := files.RemoveAll(gr.Loc.Path); err != nil {
		return err
	}

	if err := gr.Git.AddAll(); err != nil {
		return err
	}

	return gr.Git.Commit(fmt.Sprintf("[%s] %s", gr.Loc.DBName, mesg))
}

func trackRepo(gr *Repository) error {
	if gr.IsTracked() {
		return ErrGitTracked
	}

	if err := gr.Export(); err != nil {
		return err
	}

	if err := gr.Tracker.Track(gr.Loc.Hash).Save(); err != nil {
		return err
	}

	return gr.Commit("new tracking")
}

func initGPG(c *ui.Console, gr *Repository) error {
	if err := gpg.Init(gr.Git.RepoPath, AttributesFile); err != nil {
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
		fmt.Print(c.SuccessMesg("GPG repo initialized\n"))
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

// Read reads the repo and returns the bookmarks.
func Read(c *ui.Console, path string) ([]*bookmark.Bookmark, error) {
	loader := readJSONRepo
	prefix := "Loading JSON bookmarks"

	if gpg.IsInitialized(path) {
		loader = readGPGRepo
		prefix = "Decrypting GPG bookmarks"
	}

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid(prefix).StringReset()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
	)

	return loader(c, path, sp)
}

// selectAndInsert prompts the user to select records to import.
func selectAndInsert(c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return err
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithArgs("--cycle"),
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
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

	r, err := repository.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := make([]*bookmark.Bookmark, 0, len(selected))
	for i := range selected {
		bs = append(bs, &selected[i])
	}

	debookmarks := port.Deduplicate(c, r, bs)
	if err := r.InsertMany(context.Background(), debookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(debookmarks)
	if n > 0 {
		c.F.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
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
func Info(c *ui.Console, dbPath string) (string, error) {
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	if !gr.IsTracked() {
		return c.F.StringReset(), err
	}

	cfg := config.App.Git
	if !cfg.Enabled {
		return "", nil
	}

	c.F.Reset().Headerln(cri("git:"))

	sum, err := gr.Summary()
	if err != nil {
		return c.F.StringReset(), err
	}

	// remote
	if sum.GitRemote != "" {
		c.F.Rowln(txt.PaddedLine("remote:", sum.GitRemote))
	}

	// repo type
	t := cbc("JSON")
	if cfg.GPG {
		t = cbm("GPG")
	}
	c.F.Rowln(txt.PaddedLine("type:", t))

	if sum.LastSync != "" {
		tt, err := time.Parse(time.RFC3339, sum.LastSync)
		if err != nil {
			return c.F.StringReset(), err
		}

		lastSync := sum.LastSync + cgi(" ("+txt.RelativeTime(tt.Format(txt.TimeLayout))+")")
		c.F.Rowln(txt.PaddedLine("last sync:", lastSync))
		c.F.Success(txt.PaddedLine("sync:", true)).Ln()
	} else {
		c.F.Error(txt.PaddedLine("sync:", false)).Ln()
	}

	return c.F.StringReset(), nil
}

func handleOptNew(c *ui.Console, gr *Repository) (string, error) {
	if err := intoDBFromGit(c, gr); err != nil {
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptCreate(c *ui.Console, gr *Repository) (string, error) {
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
	if err := r.Init(context.Background()); err != nil {
		return "", fmt.Errorf("initializing database: %w", err)
	}

	return parseGitRepoOpt(c, opt, newGr)
}

func handleOptDrop(c *ui.Console, gr *Repository) (string, error) {
	c.Warning("Dropping database\n").Flush()
	if err := db.DropFromPath(gr.Loc.DBPath); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return handleOptMerge(c, gr)
}

func handleOptMerge(c *ui.Console, gr *Repository) (string, error) {
	c.Info("Merging database\n").Flush()
	if err := mergeAndInsert(c, gr); err != nil {
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptSelect(c *ui.Console, gr *Repository) (string, error) {
	if err := selectAndInsert(c, gr.Loc.DBPath, gr.Git.RepoPath); err != nil {
		if errors.Is(err, menu.ErrFzfActionAborted) {
			return "", nil
		}
		return "", err
	}
	return gr.Loc.DBPath, nil
}

func handleOptIgnore(c *ui.Console, gr *Repository) (string, error) {
	repoName := files.StripSuffixes(filepath.Base(gr.Loc.DBPath))
	c.ReplaceLine(c.Warning(fmt.Sprintf("%s repo %q", color.Yellow("skipping"), repoName)).StringReset())
	return "", nil
}

// intoDBFromGit loads a git repo into a database.
func intoDBFromGit(c *ui.Console, gr *Repository) error {
	bookmarks, err := extractFromGitRepo(c, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	// FIX: replace with `repository.New`
	store, err := db.Init(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := store.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	r, err := repository.New(store.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	c.F.Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), gr.Loc.DBName)).Flush()

	return nil
}

// mergeAndInsert merges non-duplicates records into database.
func mergeAndInsert(c *ui.Console, gr *Repository) error {
	r, err := repository.New(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := extractFromGitRepo(c, gr.Git.RepoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = port.Deduplicate(c, r, bookmarks)
	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
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
		c.F.Reset().Success(fmt.Sprintf("Imported %d records into %q\n", n, gr.Loc.DBName)).Flush()
	}

	return nil
}
