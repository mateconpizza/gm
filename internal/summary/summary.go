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
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	cmi = func(s string) string { return color.BrightMagenta(s).Italic().String() } // Color BrightMagenta Italic
	cgi = func(s string) string { return color.Gray(s).Italic().String() }          // Color Gray Italic
)

// Repo returns a summary of the repository.
func Repo(a *app.Context) string {
	var (
		name    = a.DB.Name()
		path    = txt.PaddedLine("path:", files.CollapseHomeDir(a.DB.Cfg.Fullpath()))
		records = txt.PaddedLine("records:", a.DB.Count(a.Ctx, "bookmarks"))
		tags    = txt.PaddedLine("tags:", a.DB.Count(a.Ctx, "tags"))
	)

	if name == config.MainDBName {
		name += cgi(" (main) ")
	}

	return a.Console.Frame.Headerln(color.Yellow(name).Italic().String()).
		Rowln(records).
		Rowln(tags).
		Rowln(path).
		StringReset()
}

// RepoFromPath returns a summary of the repository.
func RepoFromPath(a *app.Context, dbPath, backupPath string) string {
	if strings.HasSuffix(dbPath, ".enc") {
		dbPath = strings.TrimSuffix(dbPath, ".enc")
		s := cmi(filepath.Base(dbPath))

		e := "(locked)"
		if filepath.Base(dbPath) == config.MainDBName {
			e = "(main locked)"
		}

		return a.Console.Frame.Mid(txt.PaddedLine(s, cgi(e))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(dbPath))

	r, err := db.New(dbPath)
	if err != nil {
		return a.Console.Frame.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", r.Count(a.Ctx, "bookmarks"))
	tags := txt.PaddedLine("tags:", r.Count(a.Ctx, "tags"))
	name := color.Yellow(r.Name()).Italic().String()

	if r.Name() == config.MainDBName {
		name = txt.PaddedLine(name, cgi("(main)"))
	}

	a.Console.Frame.Headerln(name).Rowln(records).Rowln(tags)
	dbName := files.StripSuffixes(r.Name())
	backups, _ := files.List(backupPath, "*_"+dbName+".db*")
	if len(backups) > 0 {
		a.Console.Frame.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return a.Console.Frame.Rowln(path).StringReset()
}

// RepoRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n)
func RepoRecords(ctx context.Context, r *db.SQLite) string {
	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))
	return r.Name() + " " + cgi(main)
}

// RepoRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoRecordsFromPath(ctx context.Context, p string) string {
	if strings.HasSuffix(p, ".enc") {
		s := strings.TrimSuffix(filepath.Base(p), ".enc")
		return txt.PaddedLine(s, cgi("(locked)"))
	}

	r, _ := db.New(p)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return txt.PaddedLine(r.Name(), cgi(main))
}

// BackupWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupWithFmtDate(ctx context.Context, r *db.SQLite) string {
	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))
	t := strings.Split(r.Name(), "_")[0]

	return r.Name() + " " + cgi(main) + " " + cgi(txt.RelativeTime(t))
}

// BackupWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupWithFmtDateFromPath(ctx context.Context, p string) string {
	name := filepath.Base(p)
	t := strings.Split(name, "_")[0]
	bkTime := cgi(txt.RelativeTime(t))

	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += cgi(" (locked) ")

		return name + bkTime
	}

	r, err := db.New(p)
	if err != nil {
		slog.Warn("creating repository from path", "path", p, "error", err)
		return ""
	}
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count(ctx, "bookmarks"))

	return r.Name() + " " + cgi(main) + " " + bkTime
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(a *app.Context) string {
	backupPath := a.Cfg.Path.Backup
	dbName := files.StripSuffixes(a.DB.Name())
	fs, err := files.List(backupPath, "*_"+dbName+".db*")
	if len(fs) == 0 {
		return ""
	}

	a.Console.Frame.Header(color.BrightCyan("summary:\n").Italic().String())
	if err != nil {
		return a.Console.Frame.Row(txt.PaddedLine("found:", "n/a\n")).String()
	}

	for i := range fs {
		a.Console.Frame.Rowln(BackupWithFmtDateFromPath(a.Ctx, fs[i]))
	}

	return a.Console.Frame.StringReset()
}

// Backups returns a summary of the backups.
//
// last, path and number of backups.
func Backups(a *app.Context) string {
	backupPath := a.Cfg.Path.Backup
	var (
		empty          = "n/a"
		backupsColor   = color.BrightMagenta("backups:").Italic()
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
		lastBackup = RepoRecordsFromPath(a.Ctx, lastItem)
		s := txt.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = color.BrightGreen(s).Italic().String()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(backupPath))
	last := txt.PaddedLine("last:", lastBackup)
	lastDate := txt.PaddedLine("date:", lastBackupDate)

	return a.Console.Frame.Headerln(backupsColor.String()).
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
