package summary

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/repository"
)

var (
	cmi = func(s string) string { return color.BrightMagenta(s).Italic().String() }
	cgi = func(s string) string { return color.Gray(s).Italic().String() }
)

// Repo returns a summary of the repository.
func Repo(c *ui.Console, r repository.Repo) string {
	var (
		name    = r.Name()
		path    = txt.PaddedLine("path:", files.CollapseHomeDir(config.App.DBPath))
		records = txt.PaddedLine("records:", r.Count("bookmarks"))
		tags    = txt.PaddedLine("tags:", r.Count("tags"))
	)

	if name == config.MainDBName {
		name += cgi(" (main) ")
	}

	return c.F.Headerln(color.Yellow(name).Italic().String()).
		Rowln(records).
		Rowln(tags).
		Rowln(path).
		StringReset()
}

// RepoFromPath returns a summary of the repository.
func RepoFromPath(c *ui.Console, p string) string {
	if strings.HasSuffix(p, ".enc") {
		p = strings.TrimSuffix(p, ".enc")
		s := cmi(filepath.Base(p))

		var e string
		if filepath.Base(p) == config.MainDBName {
			e = "(main locked)"
		} else {
			e = "(locked)"
		}

		return c.F.Mid(txt.PaddedLine(s, cgi(e))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(p))

	r, err := repository.New(p)
	if err != nil {
		return c.F.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", r.Count("bookmarks"))
	tags := txt.PaddedLine("tags:", r.Count("tags"))
	name := color.Yellow(r.Name()).Italic().String()

	if r.Name() == config.MainDBName {
		name = txt.PaddedLine(name, cgi("(main)"))
	}

	c.F.Headerln(name).Rowln(records).Rowln(tags)
	dbName := files.StripSuffixes(r.Name())
	backups, _ := files.List(config.App.Path.Backup, "*_"+dbName+".db*")
	if len(backups) > 0 {
		c.F.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return c.F.Rowln(path).StringReset()
}

// RepoRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n)
func RepoRecords(r repository.Repo) string {
	main := fmt.Sprintf("(main: %d)", r.Count("bookmarks"))
	return r.Name() + " " + cgi(main)
}

// RepoRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoRecordsFromPath(p string) string {
	if strings.HasSuffix(p, ".enc") {
		s := strings.TrimSuffix(filepath.Base(p), ".enc")
		return txt.PaddedLine(s, cgi("(locked)"))
	}

	r, _ := repository.New(p)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count("bookmarks"))

	return txt.PaddedLine(r.Name(), cgi(main))
}

// BackupWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupWithFmtDate(r repository.Repo) string {
	main := fmt.Sprintf("(main: %d)", r.Count("bookmarks"))
	t := strings.Split(r.Name(), "_")[0]

	return r.Name() + " " + cgi(main) + " " + cgi(txt.RelativeTime(t))
}

// BackupWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupWithFmtDateFromPath(p string) string {
	name := filepath.Base(p)
	t := strings.Split(name, "_")[0]
	bkTime := cgi(txt.RelativeTime(t))

	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += cgi(" (locked) ")

		return name + bkTime
	}

	r, err := repository.New(p)
	if err != nil {
		slog.Warn("creating repository from path", "path", p, "error", err)
		return ""
	}
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", r.Count("bookmarks"))

	return r.Name() + " " + cgi(main) + " " + bkTime
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(c *ui.Console, r repository.Repo) string {
	dbName := files.StripSuffixes(r.Name())
	fs, err := files.List(config.App.Path.Backup, "*_"+dbName+".db*")
	if len(fs) == 0 {
		return ""
	}

	c.F.Header(color.BrightCyan("summary:\n").Italic().String())
	if err != nil {
		return c.F.Row(txt.PaddedLine("found:", "n/a\n")).String()
	}

	for i := range fs {
		c.F.Rowln(BackupWithFmtDateFromPath(fs[i]))
	}

	return c.F.StringReset()
}

// Backups returns a summary of the backups.
//
// last, path and number of backups.
func Backups(c *ui.Console, r repository.Repo) string {
	var (
		empty          = "n/a"
		backupsColor   = color.BrightMagenta("backups:").Italic()
		backupsInfo    = txt.PaddedLine("found:", empty)
		lastBackup     = empty
		lastBackupDate = empty
	)

	dbName := files.StripSuffixes(r.Name())
	fs, err := files.List(config.App.Path.Backup, "*_"+dbName+".db*")
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
		lastBackup = RepoRecordsFromPath(lastItem)
		s := txt.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = color.BrightGreen(s).Italic().String()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(config.App.Path.Backup))
	last := txt.PaddedLine("last:", lastBackup)
	lastDate := txt.PaddedLine("date:", lastBackupDate)

	return c.F.Headerln(backupsColor.String()).
		Rowln(path).
		Rowln(last).
		Rowln(lastDate).
		Rowln(backupsInfo).
		StringReset()
}

// Info returns the repository info.
func Info(c *ui.Console, r repository.Repo) string {
	s := Repo(c, r)
	s += Backups(c, r)
	s += BackupListDetail(c, r)

	return s
}
