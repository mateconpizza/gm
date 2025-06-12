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
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

// RepoSummary returns a summary of the repository.
func RepoSummary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	path := txt.PaddedLine("path:", files.ReplaceHomePath(config.App.DBPath))
	records := txt.PaddedLine("records:", CountMainRecords(r))
	tags := txt.PaddedLine("tags:", CountTagsRecords(r))
	name := r.Cfg.Name
	if name == config.DefaultDBName {
		name += color.Gray(" (default) ").Italic().String()
	}

	return f.Header(color.Yellow(name).Italic().String()).
		Ln().Row(records).
		Ln().Row(tags).
		Ln().Row(path).
		Ln().String()
}

// RepoSummaryFromPath returns a summary of the repository.
func RepoSummaryFromPath(p string) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if strings.HasSuffix(p, ".enc") {
		p = strings.TrimSuffix(p, ".enc")
		s := color.BrightMagenta(filepath.Base(p)).Italic().String()
		var e string
		if filepath.Base(p) == config.DefaultDBName {
			e = color.Gray("(default locked)").Italic().String()
		} else {
			e = color.Gray("(locked)").Italic().String()
		}

		return f.Mid(txt.PaddedLine(s, e)).Ln().String()
	}

	path := txt.PaddedLine("path:", files.ReplaceHomePath(p))
	r, err := New(p)
	if err != nil {
		return f.Row(path).String()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", CountMainRecords(r))
	tags := txt.PaddedLine("tags:", CountTagsRecords(r))
	name := color.Yellow(r.Cfg.Name).Italic().String()
	if r.Cfg.Name == config.DefaultDBName {
		name = txt.PaddedLine(name, color.Gray("(default)").Italic())
	}
	f.Header(name).Ln().Row(records).Ln().Row(tags).Ln()
	backups, _ := r.ListBackups()
	if len(backups) > 0 {
		f.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return f.Row(path).Ln().String()
}

// RepoSummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n)
func RepoSummaryRecords(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()

	return r.Name() + " " + records.String()
}

// RepoSummaryRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (main: n) | (locked)
func RepoSummaryRecordsFromPath(p string) string {
	if strings.HasSuffix(p, ".enc") {
		s := strings.TrimSuffix(filepath.Base(p), ".enc")
		e := color.Gray("(locked)").Italic().String()
		return txt.PaddedLine(s, e)
	}
	r, _ := New(p)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()

	return txt.PaddedLine(r.Name(), records)
}

// BackupSummaryWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupSummaryWithFmtDate(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()
	t := strings.Split(r.Name(), "_")[0]
	bkTime := color.Gray(txt.RelativeTime(t)).Italic()

	return r.Name() + " " + records.String() + " " + bkTime.String()
}

// BackupSummaryWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func BackupSummaryWithFmtDateFromPath(p string) string {
	name := filepath.Base(p)
	t := strings.Split(name, "_")[0]
	bkTime := color.Gray(txt.RelativeTime(t)).Italic()
	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += color.Gray(" (locked) ").Italic().String()
		return name + bkTime.String()
	}

	r, err := New(p)
	if err != nil {
		slog.Warn("creating repository from path", "path", p, "error", err)
		return ""
	}
	defer r.Close()
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()

	return r.Name() + " " + records.String() + " " + bkTime.String()
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	fs, err := r.ListBackups()
	if len(fs) == 0 {
		return ""
	}
	f.Header(color.BrightCyan("summary:\n").Italic().String())
	if err != nil {
		return f.Row(txt.PaddedLine("found:", "n/a\n")).String()
	}
	backups := slice.New[string]()
	backups.Append(fs...)

	n := backups.Len()
	backups.ForEach(func(p string) {
		if n == 1 {
			f.Footer(BackupSummaryWithFmtDateFromPath(p)).Ln()
			return
		}
		f.Row(BackupSummaryWithFmtDateFromPath(p)).Ln()
		n--
	})

	return f.String()
}

// BackupsSummary returns a summary of the backups.
//
// last, path and number of backups.
func BackupsSummary(r *SQLiteRepository) string {
	var (
		f              = frame.New(frame.WithColorBorder(color.BrightGray))
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

	return f.Header(backupsColor.String()).
		Ln().Row(path).
		Ln().Row(last).
		Ln().Row(lastDate).
		Ln().Row(backupsInfo).
		Ln().String()
}

// Info returns the repository info.
func Info(r *SQLiteRepository) string {
	s := RepoSummary(r)
	s += BackupsSummary(r)
	s += BackupListDetail(r)

	return s
}
