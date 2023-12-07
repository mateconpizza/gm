package app

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
		format.BulletLine("name", Config.Name),
		format.BulletLine("home", Path.Home),
		format.BulletLine("version", Config.Version),
	})

	return s
}

func getBackupInfo() string {
	b := format.Text("backup").Cyan().Bold().String()
	s := format.Title(b, []string{
		format.BulletLine("status", format.Text("not implemented yet").Red().String()),
	})

	return s
}

func ShowInfo(records, deleted int) string {
	name := format.Text(Config.Name).Green().Bold()
	s := fmt.Sprintf("%s:\n\t%s\n\n", name, Info.Desc)
	s += getAppInfo()
	s += getDBInfo(records, deleted)
	s += getBackupInfo()

	return s
}
