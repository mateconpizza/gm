package db

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var (
	cmi = func(s string) string { return color.BrightMagenta(s).Italic().String() }
	cgi = func(s string) string { return color.Gray(s).Italic().String() }
)

// RepoSummary returns a summary of the repository.
func RepoSummary(c *ui.Console, r *SQLiteRepository) string {
	path := txt.PaddedLine("path:", files.CollapseHomeDir(config.App.DBPath))
	records := txt.PaddedLine("records:", CountMainRecords(r))
	tags := txt.PaddedLine("tags:", CountTagsRecords(r))
	name := r.Cfg.Name
	if name == config.DefaultDBName {
		name += cgi(" (default) ")
	}

	return c.F.Headerln(color.Yellow(name).Italic().String()).
		Rowln(records).
		Rowln(tags).
		Rowln(path).
		String()
}

// RepoSummaryFromPath returns a summary of the repository.
func RepoSummaryFromPath(c *ui.Console, p string) string {
	if strings.HasSuffix(p, ".enc") {
		p = strings.TrimSuffix(p, ".enc")
		s := cmi(filepath.Base(p))
		var e string
		if filepath.Base(p) == config.DefaultDBName {
			e = "(default locked)"
		} else {
			e = "(locked)"
		}

		return c.F.Mid(txt.PaddedLine(s, cgi(e))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(p))
	r, err := New(p)
	if err != nil {
		return c.F.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", CountMainRecords(r))
	tags := txt.PaddedLine("tags:", CountTagsRecords(r))
	name := color.Yellow(r.Cfg.Name).Italic().String()
	if r.Cfg.Name == config.DefaultDBName {
		name = txt.PaddedLine(name, cgi("(default)"))
	}
	c.F.Headerln(name).Rowln(records).Rowln(tags)
	backups, _ := r.ListBackups()
	if len(backups) > 0 {
		c.F.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return c.F.Rowln(path).String()
}

// RepoSummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n)
func RepoSummaryRecords(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	return r.Name() + " " + cgi(main)
}

// RepoSummaryRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoSummaryRecordsFromPath(p string) string {
	if strings.HasSuffix(p, ".enc") {
		s := strings.TrimSuffix(filepath.Base(p), ".enc")
		return txt.PaddedLine(s, cgi("(locked)"))
	}

	r, _ := New(p)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))

	return txt.PaddedLine(r.Name(), cgi(main))
}

// BackupSummaryWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupSummaryWithFmtDate(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	t := strings.Split(r.Name(), "_")[0]

	return r.Name() + " " + cgi(main) + " " + cgi(txt.RelativeTime(t))
}

// BackupSummaryWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupSummaryWithFmtDateFromPath(p string) string {
	name := filepath.Base(p)
	t := strings.Split(name, "_")[0]
	bkTime := cgi(txt.RelativeTime(t))
	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += cgi(" (locked) ")
		return name + bkTime
	}

	r, err := New(p)
	if err != nil {
		slog.Warn("creating repository from path", "path", p, "error", err)
		return ""
	}
	defer r.Close()
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))

	return r.Name() + " " + cgi(main) + " " + bkTime
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(c *ui.Console, r *SQLiteRepository) string {
	fs, err := r.ListBackups()
	if len(fs) == 0 {
		return ""
	}
	c.F.Header(color.BrightCyan("summary:\n").Italic().String())
	if err != nil {
		return c.F.Row(txt.PaddedLine("found:", "n/a\n")).String()
	}
	backups := slice.New[string]()
	backups.Append(fs...)

	n := backups.Len()
	backups.ForEach(func(p string) {
		if n == 1 {
			c.F.Footer(BackupSummaryWithFmtDateFromPath(p)).Ln()
			return
		}
		c.F.Row(BackupSummaryWithFmtDateFromPath(p)).Ln()
		n--
	})

	return c.F.String()
}

// BackupsSummary returns a summary of the backups.
//
// last, path and number of backups.
func BackupsSummary(c *ui.Console, r *SQLiteRepository) string {
	var (
		empty          = "n/a"
		backupsColor   = color.BrightMagenta("backups:").Italic()
		backupsInfo    = txt.PaddedLine("found:", empty)
		lastBackup     = empty
		lastBackupDate = empty
	)

	var n int
	fs, err := r.ListBackups()
	if len(fs) == 0 {
		return ""
	}
	backups := slice.New[string]()
	backups.Append(fs...)
	if err != nil {
		n = 0
	} else {
		n = backups.Len()
	}

	if n > 0 {
		backupsInfo = txt.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastItem := backups.Item(n - 1)
		lastBackup = RepoSummaryRecordsFromPath(lastItem)
		s := txt.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = color.BrightGreen(s).Italic().String()
	}
	path := txt.PaddedLine("path:", config.App.Path.Backup)
	last := txt.PaddedLine("last:", lastBackup)
	lastDate := txt.PaddedLine("date:", lastBackupDate)

	return c.F.Headerln(backupsColor.String()).
		Rowln(path).
		Rowln(last).
		Rowln(lastDate).
		Rowln(backupsInfo).
		String()
}

// Info returns the repository info.
func Info(c *ui.Console, r *SQLiteRepository) string {
	s := RepoSummary(c, r)
	s += BackupsSummary(c, r)
	s += BackupListDetail(c, r)

	return s
}
