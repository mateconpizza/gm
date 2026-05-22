package git

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

const JSONFileExt = ".json"

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
	r, err := db.New(ctx, dbPath)
	if err != nil {
		return err
	}

	rs, err := db.NewStats(ctx, r)
	if err != nil {
		return err
	}

	summary.RepoStats = rs
	summary.GenChecksum()

	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(ctx context.Context, gr *Repository, actionMsg string) error {
	var err error
	err = writeRepoStats(ctx, gr)
	if err != nil {
		return err
	}

	gm := gr.Git
	// check if any changes
	changed, _ := gm.hasChanges()
	if !changed {
		return nil
	}

	if err = gm.AddAll(); err != nil {
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
	r, err := db.New(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	bs, err := r.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting bookmarks: %w", err)
	}

	return bs, nil
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

	return Commit(gr.Git.ctx, gr.Git.repoPath, mesg)
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

	return gr.Commit("add tracking")
}

func initGPG(c *ui.Console, gr *Repository, k *gpg.Fingerprint) error {
	if !k.IsTrusted() {
		return fmt.Errorf("%w: %s", gpg.ErrKeyNotTrusted, k.UserID)
	}

	if err := gpg.Init(gr.Git.repoPath, AttributesFile, k); err != nil {
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
		fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("GPG repo initialized with key %q", k.UserID)))
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

func repoStatus(c *ui.Console, gr *Repository) string {
	var (
		sb strings.Builder
		t  string
		p  = c.Palette()
	)

	if !gr.IsTracked() {
		sb.WriteString(txt.PaddedLine(gr.Loc.Name, p.Gray.Wrap("(not tracked)\n", p.Italic)))
		return c.Error(sb.String()).StringReset()
	}

	t = p.BrightMagenta.Wrap("json ", p.Bold)
	if gr.IsEncrypted() {
		t = p.BrightMagenta.Wrap("gpg ", p.Bold)
	}

	name := gr.Loc.Name
	if name == files.StripSuffixes(application.MainDBName) {
		name = "main"
	}

	s := strings.TrimSpace(fmt.Sprintf("(%s)", gr.String()))
	sb.WriteString(txt.PaddedLine(name, t+p.Gray.Wrap(s, p.Italic)))

	return c.Success(sb.String() + "\n").String()
}

func StatusRepo(c *ui.Console, dbPath string) (string, error) {
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	return repoStatus(c, gr), nil
}

// Info returns a prettify info of the repository.
func Info(c *ui.Console, dbPath string, cfg *application.Git) (string, error) {
	f, p := c.Frame(), c.Palette()
	gr, err := NewRepo(dbPath)
	if err != nil {
		return "", err
	}

	if !gr.IsTracked() {
		return f.StringReset(), err
	}

	if !cfg.Enabled {
		return "", nil
	}

	f.Reset().Headerln(p.BrightRed.Wrap("git:", p.Italic))

	sum, err := gr.Summary()
	if err != nil {
		return f.StringReset(), err
	}

	// remote
	if sum.GitRemote != "" {
		f.Rowln(txt.PaddedLine("remote:", sum.GitRemote))
	}

	// repo type
	t := p.BrightCyan.Wrap("JSON", p.Bold)
	if cfg.GPG {
		t = p.BrightMagenta.Wrap("GPG", p.Bold)
	}
	f.Rowln(txt.PaddedLine("type:", t))

	if sum.LastSync != "" {
		tt, err := time.Parse(time.RFC3339, sum.LastSync)
		if err != nil {
			return f.StringReset(), err
		}

		lastSync := txt.RelativeTime(tt.Format(txt.TimeLayout)) + p.Gray.With(p.Italic).
			Sprintf(" (%s)", sum.LastSync)
		f.Rowln(txt.PaddedLine("last sync:", lastSync))
		f.Success(txt.PaddedLine("sync:", true)).Ln()
	} else {
		f.Error(txt.PaddedLine("sync:", false)).Ln()
	}

	return f.StringReset(), nil
}
