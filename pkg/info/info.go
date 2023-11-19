package info

import (
	"fmt"
	"strconv"

	"gomarks/pkg/config"
	"gomarks/pkg/database"
)

func FormatBulletLine(label, value string) string {
	return fmt.Sprintf("    %s %-15s: %s\n", config.BulletPoint, label, value)
}

func FormatTitle(title string, items []string) string {
	var s string

	t := fmt.Sprintf("> %s:\n", title)
	s += t

	for _, item := range items {
		s += item
	}

	return s
}

func getDBInfo(r *database.SQLiteRepository) string {
	records, err := r.GetRecordsLength(config.DB.Table.Main)
	if err != nil {
		return ""
	}

	deleted, err := r.GetRecordsLength(config.DB.Table.Deleted)
	if err != nil {
		return ""
	}

	s := FormatTitle("database", []string{
		FormatBulletLine("path", config.DB.Path),
		FormatBulletLine("records", strconv.Itoa(records)),
		FormatBulletLine("deleted", strconv.Itoa(deleted)),
	})

	return s
}

func getAppInfo() string {
	s := FormatTitle("info", []string{
		FormatBulletLine("name", config.App.Name),
		FormatBulletLine("home", config.Path.Home),
		FormatBulletLine("version", config.App.Info.Version),
	})

	return s
}

func getBackupInfo() string {
	s := FormatTitle("backup", []string{
		FormatBulletLine("status", "not implemented yet"),
	})

	return s
}

func AppInfo(r *database.SQLiteRepository) string {
	s := fmt.Sprintf("App: %s\n\n", config.App.Name)
	s += getAppInfo()
	s += getDBInfo(r)
	s += getBackupInfo()

	return s
}
