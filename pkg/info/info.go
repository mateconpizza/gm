package info

import (
	"fmt"
	"strconv"

	c "gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/util"
)

func FormatBulletLine(label string, value string) string {
	return fmt.Sprintf("    %s %-15s: %s\n", c.BulletPoint, label, value)
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
	records, err := r.GetRecordsLength(c.DBMainTableName)
	if err != nil {
		return ""
	}
	deleted, err := r.GetRecordsLength(c.DBDeletedTableName)
	if err != nil {
		return ""
	}
	s := FormatTitle("database", []string{
		FormatBulletLine("path", util.GetDBPath()),
		FormatBulletLine("records", strconv.Itoa(records)),
		FormatBulletLine("deleted", strconv.Itoa(deleted)),
	})
	return s
}

func getAppInfo() string {
	s := FormatTitle("info", []string{
		FormatBulletLine("name", c.AppName),
		FormatBulletLine("home", util.GetAppHome()),
		FormatBulletLine("version", c.Version),
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
	s := fmt.Sprintf("App: %s\n\n", c.AppName)
	s += getAppInfo()
	s += getDBInfo(r)
	s += getBackupInfo()
	return s
}
