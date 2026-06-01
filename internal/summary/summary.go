// Package summary provides repository and backup summary generation.
// It formats database metadata, statistics, and backup information for display.
package summary

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// Repo returns a summary of the repository.
func Repo(ctx context.Context, d *deps.Deps) (string, error) {
	r, err := d.Repository()
	if err != nil {
		return "", err
	}

	stats, err := db.NewStats(ctx, r)
	if err != nil {
		return "", err
	}

	stats.Name = r.Name()

	p := d.Console().Palette()

	name := r.Name()
	if name == application.MainDBName {
		name += p.Gray.Wrap(" (main) ", p.Italic)
	}

	f := d.Console().Frame()
	f.Headerln(p.Yellow.Wrap(name, p.Italic)).
		Rowln(txt.PaddedLine("records:", stats.Bookmarks)).
		Rowln(txt.PaddedLine("tags:", stats.Tags))

	if stats.Favorites > 0 {
		f.Rowln(txt.PaddedLine("favorites:", stats.Favorites))
	}
	if stats.DeadLinks > 0 {
		f.Rowln(txt.PaddedLine("deadlinks:", stats.DeadLinks))
	}
	if stats.TotalVisits > 0 {
		f.Rowln(txt.PaddedLine("visits:", stats.TotalVisits))
	}

	f.Rowln(txt.PaddedLine("path:", files.CollapseHomeDir(r.Cfg.Fullpath())))

	return f.StringReset(), nil
}

// RepoFromPath returns a summary of the repository.
func RepoFromPath(ctx context.Context, d *deps.Deps, dbPath, backupPath string) string {
	f, p := d.Console().Frame(), d.Console().Palette()
	if base, found := strings.CutSuffix(dbPath, ".enc"); found {
		dbPath = base
		s := p.BrightMagenta.Wrap(filepath.Base(dbPath), p.Italic)

		e := "(locked)"
		if filepath.Base(dbPath) == application.MainDBName {
			e = "(main locked)"
		}

		return f.Mid(txt.PaddedLine(s, p.Gray.Wrap(e, p.Italic))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(dbPath))

	r, err := db.New(ctx, dbPath)
	if err != nil {
		return f.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", r.Count(ctx, "bookmarks"))
	tags := txt.PaddedLine("tags:", r.Count(ctx, "tags"))
	name := p.Yellow.Wrap(r.Name(), p.Italic)

	if r.Name() == application.MainDBName {
		name = txt.PaddedLine(name, p.Gray.Wrap("(main)", p.Italic))
	}

	f.Headerln(name).Rowln(records).Rowln(tags)
	dbName := files.StripSuffixes(r.Name())
	backups, _ := files.List(backupPath, "*_"+dbName+".db*")
	if len(backups) > 0 {
		f.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return f.Rowln(path).StringReset()
}

// RepoRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoRecordsFromPath(ctx context.Context, c *ui.Console, fp string) string {
	p := c.Palette()
	if strings.HasSuffix(fp, ".enc") {
		s := strings.TrimSuffix(filepath.Base(fp), ".enc")
		return txt.PaddedLine(s, p.Gray.Wrap("(locked)", p.Italic))
	}

	r, _ := db.New(ctx, fp)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return txt.PaddedLine(r.Name(), p.Gray.Wrap(main, p.Italic))
}

// BackupWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupWithFmtDate(ctx context.Context, c *ui.Console, r *db.SQLite) string {
	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))
	t, _, _ := strings.Cut(r.Name(), "_")
	p := c.Palette()

	return r.Name() + " " + p.Gray.Wrap(main, p.Italic) + " " + p.Gray.Wrap(txt.RelativeTime(t), p.Italic)
}

// BackupWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupWithFmtDateFromPath(ctx context.Context, c *ui.Console, fp string) string {
	p := c.Palette()
	name := filepath.Base(fp)
	t, _, _ := strings.Cut(name, "_")
	bkTime := p.Gray.Wrap(txt.RelativeTime(t), p.Italic)

	if base, found := strings.CutSuffix(name, ".enc"); found {
		name = base + p.Gray.Wrap(" (locked) ", p.Italic)
		return name + bkTime
	}

	r, err := db.New(ctx, fp)
	if err != nil {
		slog.Warn("creating repository from path", "path", fp, "error", err)
		return ""
	}
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return r.Name() + " " + p.Gray.Wrap(main, p.Italic) + " " + bkTime
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(ctx context.Context, d *deps.Deps, complete bool) (string, error) {
	const maxItems = 3

	r, err := d.Repository()
	if err != nil {
		return "", err
	}

	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	c, p := d.Console(), d.Console().Palette()
	backupPath := app.Path.Backup
	dbName := files.StripSuffixes(r.Name())
	fs, err := files.List(backupPath, "*_"+dbName+".db*")
	if len(fs) == 0 {
		return "", nil
	}

	f := c.Frame()

	f.Header(p.BrightCyan.Wrap("summary:\n", p.Italic))
	if err != nil {
		return f.Row(txt.PaddedLine("found:", "n/a\n")).String(), nil
	}

	if len(fs) > maxItems && !complete {
		f.Rowln(p.Gray.Sprintf("... %d more", len(fs)-maxItems))
		fs = fs[len(fs)-maxItems:]
	}
	for i := range fs {
		f.Rowln(BackupWithFmtDateFromPath(ctx, d.Console(), fs[i]))
	}

	return f.StringReset(), nil
}

// Backups returns a summary of the backups.
//
// last, path and number of backups.
func Backups(ctx context.Context, d *deps.Deps) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	var (
		p              = d.Console().Palette()
		backupPath     = app.Path.Backup
		empty          = "n/a"
		backupsColor   = p.BrightMagenta.Wrap("backups:", p.Italic)
		backupsInfo    = txt.PaddedLine("found:", empty)
		lastBackup     = empty
		lastBackupDate = empty
	)

	fs, err := files.List(backupPath, "*_"+app.DBNameBase()+".db*")
	if len(fs) == 0 {
		return "", nil
	}

	var n int
	if err != nil {
		n = 0
	} else {
		n = len(fs)
	}

	if n > 0 {
		backupsInfo = txt.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastItem := fs[n-1]
		lastBackup = RepoRecordsFromPath(ctx, d.Console(), lastItem)
		s := txt.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = p.BrightGreen.Wrap(s, p.Italic)
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(backupPath))
	last := txt.PaddedLine("last:", lastBackup)
	lastDate := txt.PaddedLine("date:", lastBackupDate)

	return d.Console().Frame().Headerln(backupsColor).
		Rowln(path).
		Rowln(last).
		Rowln(lastDate).
		Rowln(backupsInfo).
		StringReset(), nil
}

// Info returns the repository info.
func Info(ctx context.Context, d *deps.Deps) (string, error) {
	var sb strings.Builder

	fn := func(s string, err error) error {
		if err != nil {
			return err
		}

		sb.WriteString(s)
		return nil
	}

	if err := fn(Repo(ctx, d)); err != nil {
		return "", err
	}
	if err := fn(Backups(ctx, d)); err != nil {
		return "", err
	}
	if err := fn(BackupListDetail(ctx, d, false)); err != nil {
		return "", err
	}

	return sb.String(), nil
}
