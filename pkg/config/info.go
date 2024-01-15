// Copyright © 2023 haaag <git.haaag@gmail.com>
package config

import (
	"fmt"
	"runtime"
	"strconv"

	"gomarks/pkg/format"
)

func dbInfo(records, deleted int) string {
	t := format.Text("database").Yellow().Bold().String()
	s := format.HeaderWithSection(t, []string{
		format.BulletLine("path", DB.Path),
		format.BulletLine("records", strconv.Itoa(records)),
		format.BulletLine("deleted", strconv.Itoa(deleted)),
	})

	return s
}

func appInfo() string {
	i := format.Text("info").Blue().Bold().String()
	s := format.HeaderWithSection(i, []string{
		format.BulletLine("name", App.Name),
		format.BulletLine("home", App.Path.Home),
		format.BulletLine("version", App.Version),
	})

	return s
}

func backupInfo() string {
	notImplementedYet := format.Text("not implemented yet").Red().Bold().String()
	b := format.Text("backup").Cyan().Bold().String()
	s := format.HeaderWithSection(b, []string{
		format.BulletLine("status", notImplementedYet),
	})

	return s
}

func Info(records, deleted int) string {
	name := format.Text(App.Name).Green().Bold()
	s := fmt.Sprintf("%s v%s:\n%s\n", name, App.Version, format.BulletLine(App.Data.Desc, ""))
	s += appInfo()
	s += dbInfo(records, deleted)
	s += backupInfo()
	return s
}

func Version() {
	name := format.Text(App.Name).Blue().Bold()
	fmt.Printf("%s v%s %s/%s\n", name, App.Version, runtime.GOOS, runtime.GOARCH)
}
