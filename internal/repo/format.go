package repo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
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

// BackupSummaryDetail returns the details of a backup.
func BackupSummaryDetail(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("summary:\n").Italic().String())

	backups, err := Backups(r)
	if err != nil {
		return f.Row(format.PaddedLine("found:", "n/a\n")).String()
	}
	defer backups.ForEachMut(func(bk *SQLiteRepository) { bk.Close() })

	n := backups.Len()
	backups.ForEachMut(func(bk *SQLiteRepository) {
		if n == 1 {
			f.Footer(BackupSummaryWithFmtDate(bk)).Ln()
			return
		}
		f.Row(BackupSummaryWithFmtDate(bk)).Ln()
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
	backups, err := Backups(r)
	if err != nil {
		n = 0
	} else {
		n = backups.Len()
	}

	if n > 0 {
		backupsInfo = format.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastItem := backups.Item(n - 1)
		lastBackup = RepoSummaryRecords(&lastItem)
		s := format.RelativeTime(strings.Split(lastItem.Name(), "_")[0])
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
	s += BackupSummaryDetail(r)

	return s
}
