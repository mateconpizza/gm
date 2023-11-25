/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/

package config

type ConfigApp struct {
	Name string      `json:"name"`
	Info Info        `json:"data"`
	Env  Environment `json:"environment"`
}

type Database struct {
	Name  string   `json:"name"`
	Table DBTables `json:"tables"`
	Path  string   `json:"db_path"`
}

type DBTables struct {
	Main    string `json:"main"`
	Temp    string `json:"temp"`
	Deleted string `json:"deleted"`
}

type FilePath struct {
	Home   string `json:"config_dir"`
	Backup string `json:"backup_dir"`
}

type Info struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Tags    string `json:"tags"`
	Desc    string `json:"desc"`
	Version string `json:"version"`
}

type Environment struct {
	Home   string `json:"var_home"`
	Editor string `json:"var_editor"`
}

var App = ConfigApp{
	Name: "gomarks",
	Info: Info{
		URL:     "https://github.com/haaag/GoMarks#readme",
		Title:   "Gomarks",
		Tags:    "golang,awesome,bookmarks",
		Desc:    "Simple yet powerful bookmark manager for your terminal",
		Version: "0.0.2",
	},
	Env: Environment{
		Home:   "GOMARKS_HOME",
		Editor: "GOMARKS_EDITOR",
	},
}

var DB = Database{
	Name: "bookmarks.db",
	Table: DBTables{
		Main:    "bookmarks",
		Temp:    "temp_bookmarks",
		Deleted: "deleted_bookmarks",
	},
	Path: "",
}

var Path = FilePath{
	Home:   "",
	Backup: "",
}

var Editors = []string{"vim", "nvim", "nano", "emacs", "helix"}

var BulletPoint string = "\u2022"
