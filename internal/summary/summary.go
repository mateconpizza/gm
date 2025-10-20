package summary

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// Repo returns a summary of the repository.
func Repo(a *app.Context) string {
	var (
		name    = a.DB.Name()
		path    = txt.PaddedLine("path:", files.CollapseHomeDir(a.DB.Cfg.Fullpath()))
		records = txt.PaddedLine("records:", a.DB.Count(a.Ctx, "bookmarks"))
		tags    = txt.PaddedLine("tags:", a.DB.Count(a.Ctx, "tags"))
		p       = a.Console().Palette()
	)

	if name == config.MainDBName {
		name += p.GrayItalic(" (main) ")
	}

	return a.Console().Frame().Headerln(p.YellowItalic(name)).
		Rowln(records).
		Rowln(tags).
		Rowln(path).
		StringReset()
}

// RepoFromPath returns a summary of the repository.
func RepoFromPath(a *app.Context, dbPath, backupPath string) string {
	f, p := a.Console().Frame(), a.Console().Palette()
	if strings.HasSuffix(dbPath, ".enc") {
		dbPath = strings.TrimSuffix(dbPath, ".enc")
		s := p.BrightMagentaItalic(filepath.Base(dbPath))

		e := "(locked)"
		if filepath.Base(dbPath) == config.MainDBName {
			e = "(main locked)"
		}

		return f.Mid(txt.PaddedLine(s, p.GrayItalic(e))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(dbPath))

	r, err := db.New(dbPath)
	if err != nil {
		return f.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", r.Count(a.Ctx, "bookmarks"))
	tags := txt.PaddedLine("tags:", r.Count(a.Ctx, "tags"))
	name := p.YellowItalic(r.Name())

	if r.Name() == config.MainDBName {
		name = txt.PaddedLine(name, p.GrayItalic("(main)"))
	}

	f.Headerln(name).Rowln(records).Rowln(tags)
	dbName := files.StripSuffixes(r.Name())
	backups, _ := files.List(backupPath, "*_"+dbName+".db*")
	if len(backups) > 0 {
		f.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return f.Rowln(path).StringReset()
}

// RepoRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n)
func RepoRecords(ctx context.Context, c *ui.Console, r *db.SQLite) string {
	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))
	return r.Name() + " " + c.Palette().GrayItalic(main)
}

// RepoRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoRecordsFromPath(ctx context.Context, c *ui.Console, fp string) string {
	p := c.Palette()
	if strings.HasSuffix(fp, ".enc") {
		s := strings.TrimSuffix(filepath.Base(fp), ".enc")
		return txt.PaddedLine(s, p.GrayItalic("(locked)"))
	}

	r, _ := db.New(fp)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return txt.PaddedLine(r.Name(), p.GrayItalic(main))
}

// BackupWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupWithFmtDate(ctx context.Context, c *ui.Console, r *db.SQLite) string {
	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))
	t := strings.Split(r.Name(), "_")[0]
	p := c.Palette()

	return r.Name() + " " + p.GrayItalic(main) + " " + p.GrayItalic(txt.RelativeTime(t))
}

// BackupWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupWithFmtDateFromPath(ctx context.Context, c *ui.Console, fp string) string {
	p := c.Palette()
	name := filepath.Base(fp)
	t := strings.Split(name, "_")[0]
	bkTime := p.GrayItalic(txt.RelativeTime(t))

	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += p.GrayItalic(" (locked) ")

		return name + bkTime
	}

	r, err := db.New(fp)
	if err != nil {
		slog.Warn("creating repository from path", "path", fp, "error", err)
		return ""
	}
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return r.Name() + " " + p.GrayItalic(main) + " " + bkTime
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(a *app.Context) string {
	c := a.Console()
	p := c.Palette()
	backupPath := a.Cfg.Path.Backup
	dbName := files.StripSuffixes(a.DB.Name())
	fs, err := files.List(backupPath, "*_"+dbName+".db*")
	if len(fs) == 0 {
		return ""
	}

	f := c.Frame()

	f.Header(p.BrightCyanItalic("summary:\n"))
	if err != nil {
		return f.Row(txt.PaddedLine("found:", "n/a\n")).String()
	}

	for i := range fs {
		f.Rowln(BackupWithFmtDateFromPath(a.Ctx, a.Console(), fs[i]))
	}

	return f.StringReset()
}

// Backups returns a summary of the backups.
//
// last, path and number of backups.
func Backups(a *app.Context) string {
	var (
		p              = a.Console().Palette()
		backupPath     = a.Cfg.Path.Backup
		empty          = "n/a"
		backupsColor   = p.BrightMagentaItalic("backups:")
		backupsInfo    = txt.PaddedLine("found:", empty)
		lastBackup     = empty
		lastBackupDate = empty
	)

	dbName := files.StripSuffixes(a.DB.Name())
	fs, err := files.List(backupPath, "*_"+dbName+".db*")
	if len(fs) == 0 {
		return ""
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
		lastBackup = RepoRecordsFromPath(a.Ctx, a.Console(), lastItem)
		s := txt.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = p.BrightGreenItalic(s)
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(backupPath))
	last := txt.PaddedLine("last:", lastBackup)
	lastDate := txt.PaddedLine("date:", lastBackupDate)

	return a.Console().Frame().Headerln(backupsColor).
		Rowln(path).
		Rowln(last).
		Rowln(lastDate).
		Rowln(backupsInfo).
		StringReset()
}

// Info returns the repository info.
func Info(a *app.Context) string {
	s := Repo(a)
	s += Backups(a)
	s += BackupListDetail(a)

	return s
}
