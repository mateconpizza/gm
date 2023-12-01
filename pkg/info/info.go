package info

import (
	"fmt"
	"strconv"

	"gomarks/pkg/config"
	"gomarks/pkg/format"
)

func getDBInfo(records, deleted int) string {
	s := format.Title("database", []string{
		format.BulletLine("path", config.DB.Path),
		format.BulletLine("records", strconv.Itoa(records)),
		format.BulletLine("deleted", strconv.Itoa(deleted)),
	})

	return s
}

func getAppInfo() string {
	s := format.Title("info", []string{
		format.BulletLine("name", config.App.Name),
		format.BulletLine("home", config.Path.Home),
		format.BulletLine("version", config.App.Info.Version),
	})

	return s
}

func getBackupInfo() string {
	s := format.Title("backup", []string{
		format.BulletLine("status", format.Error("not implemented yet")),
	})

	return s
}

func Show(records, deleted int) string {
	s := fmt.Sprintf("%s:\n\t%s\n\n", config.App.Name, config.App.Desc)
	s += getAppInfo()
	s += getDBInfo(records, deleted)
	s += getBackupInfo()

	return s
}
