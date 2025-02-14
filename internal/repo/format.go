package repo

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
)

// RepoSummary returns a summary of the repository.
func RepoSummary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	path := format.PaddedLine("path:", r.Cfg.Fullpath())
	records := format.PaddedLine("records:", CountRecords(r, r.Cfg.Tables.Main))
	deleted := format.PaddedLine("deleted:", CountRecords(r, r.Cfg.Tables.Deleted))
	tags := format.PaddedLine("tags:", CountRecords(r, r.Cfg.Tables.Tags))

	return f.Header(color.Yellow(r.Cfg.Name).Italic().String()).
		Ln().Row(records).
		Ln().Row(deleted).
		Ln().Row(tags).
		Ln().Row(path).
		Ln().String()
}

// RepoSummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
func RepoSummaryRecords(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d, ", CountRecords(r, r.Cfg.Tables.Main))
	deleted := fmt.Sprintf("deleted: %d)", CountRecords(r, r.Cfg.Tables.Deleted))
	records := color.Gray(main + deleted).Italic()

	return r.Cfg.Name + " " + records.String()
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
			f.Footer(RepoSummaryRecords(bk)).Ln()
			return
		}
		f.Row(RepoSummaryRecords(bk)).Ln()
		n--
	})

	return f.String()
}

// SummaryBackupLine returns a summary of a backup in one line.
func SummaryBackupLine(r *SQLiteRepository) string {
	main := fmt.Sprintf("(main: %d, ", CountRecords(r, r.Cfg.Tables.Main))
	deleted := fmt.Sprintf("deleted: %d)", CountRecords(r, r.Cfg.Tables.Deleted))
	records := color.Gray(main + deleted).Italic()

	return filepath.Base(r.Cfg.Fullpath()) + " " + records.String()
}

// BackupsSummary returns a summary of the backups.
//
// last, path and number of backups.
func BackupsSummary(r *SQLiteRepository) string {
	var (
		f            = frame.New(frame.WithColorBorder(color.BrightGray))
		empty        = "n/a"
		backupsColor = color.BrightMagenta("backups:").Italic()
		backupsInfo  = format.PaddedLine("found:", empty)
		lastBackup   = empty
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
	}
	path := format.PaddedLine("path:", r.Cfg.Backup.Path)
	last := format.PaddedLine("last:", lastBackup)

	return f.Header(backupsColor.String()).
		Ln().Row(last).
		Ln().Row(path).
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
