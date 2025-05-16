package repo

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
)

// RepoSummary returns a summary of the repository.
func RepoSummary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	path := format.PaddedLine("path:", r.Cfg.Fullpath())
	records := format.PaddedLine("records:", CountMainRecords(r))
	tags := format.PaddedLine("tags:", CountTagsRecords(r))
	name := r.Cfg.Name
	if name == config.DefaultDBName {
		name += color.BrightGray(" (default) ").Italic().String()
	}

	return f.Header(color.Yellow(name).Italic().String()).
		Ln().Row(records).
		Ln().Row(tags).
		Ln().Row(path).
		Ln().String()
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
//	repositoryName (main: n) | (encrypted)
func RepoSummaryRecordsFromPath(p string) string {
	if strings.HasSuffix(p, ".enc") {
		s := filepath.Base(p)
		s += color.Gray(" (encrypted)").Italic().String()
		return s
	}
	cfg, _ := NewSQLiteCfg(p)
	r, _ := New(cfg)
	defer r.Close()

	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()

	return r.Name() + " " + records.String()
}

// BackupSummaryWithFmtDate generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) (time)
func BackupSummaryWithFmtDate(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()
	t := strings.Split(r.Name(), "_")[0]
	bkTime := color.Gray(format.RelativeTime(t)).Italic()

	return r.Name() + " " + records.String() + " " + bkTime.String()
}

// BackupSummaryWithFmtDateFromPath generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (encrypted) (time)
func BackupSummaryWithFmtDateFromPath(p string) string {
	name := filepath.Base(p)
	t := strings.Split(name, "_")[0]
	bkTime := color.Gray(format.RelativeTime(t)).Italic()
	if strings.HasSuffix(name, ".enc") {
		name = strings.TrimSuffix(name, ".enc")
		name += color.Gray(" (encrypted) ").Italic().String()
		return name + bkTime.String()
	}

	cfg, _ := NewSQLiteCfg(p)
	r, _ := New(cfg)
	defer r.Close()
	main := fmt.Sprintf("(main: %d)", CountMainRecords(r))
	records := color.Gray(main).Italic()

	return r.Name() + " " + records.String() + " " + bkTime.String()
}

// BackupListDetail returns the details of a backup.
func BackupListDetail(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("summary:\n").Italic().String())

	fs, err := r.BackupsList()
	if err != nil {
		_ = fs
		return f.Row(format.PaddedLine("found:", "n/a\n")).String()
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
		backupsInfo    = format.PaddedLine("found:", empty)
		lastBackup     = empty
		lastBackupDate = empty
	)

	var n int
	fs, err := r.BackupsList()
	backups := slice.New[string]()
	backups.Append(fs...)
	if err != nil {
		n = 0
	} else {
		n = backups.Len()
	}

	if n > 0 {
		backupsInfo = format.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastItem := backups.Item(n - 1)
		lastBackup = RepoSummaryRecordsFromPath(lastItem)
		s := format.RelativeTime(strings.Split(filepath.Base(lastBackup), "_")[0])
		lastBackupDate = color.BrightGreen(s).Italic().String()
	}
	path := format.PaddedLine("path:", r.Cfg.BackupDir)
	last := format.PaddedLine("last:", lastBackup)
	lastDate := format.PaddedLine("date:", lastBackupDate)

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
