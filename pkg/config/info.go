package config

import (
	"fmt"
	"strconv"

	"gomarks/pkg/format"
)

func getDBInfo(records, deleted int) string {
	d := format.Text("database").Yellow().Bold()
	s := format.Title(d.String(), []string{
		format.BulletLine("path", DB.Path),
		format.BulletLine("records", strconv.Itoa(records)),
		format.BulletLine("deleted", strconv.Itoa(deleted)),
	})

	return s
}

func getAppInfo() string {
	i := format.Text("info").Blue().Bold().String()
	s := format.Title(i, []string{
		format.BulletLine("name", App.Name),
		format.BulletLine("home", Path.Home),
		format.BulletLine("version", App.Version),
	})

	return s
}

func getBackupInfo() string {
	notImplementedYet := format.Text("not implemented yet").Red().Bold().String()
	b := format.Text("backup").Cyan().Bold().String()
	s := format.Title(b, []string{
		format.BulletLine("status", notImplementedYet),
	})

	return s
}

func ShowInfo(records, deleted int) string {
	name := format.Text(Info.Title).Green().Bold()
	s := fmt.Sprintf("%s v%s:\n%s\n", name, App.Version, format.BulletLine(Info.Desc, ""))
	s += getAppInfo()
	s += getDBInfo(records, deleted)
	s += getBackupInfo()
	return s
}
