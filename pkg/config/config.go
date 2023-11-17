/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package config

type ConfigApp struct {
	Name string      `json:"name"`
	Info Info        `json:"data"`
	Env  Environment `json:"environment"`
}

type Database struct {
	Name         string `json:"name"`
	MainTable    string `json:"main_table"`
	TempTable    string `json:"temp_table"`
	DeletedTable string `json:"deleted_table"`
	Path         string `json:"db_path"`
}

type FilePath struct {
	ConfigDir string `json:"config_dir"`
	DataDir   string `json:"data_dir"`
	LogDir    string `json:"log_dir"`
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
	Name:         "bookmarks.db",
	MainTable:    "bookmarks",
	TempTable:    "temp_bookmarks",
	DeletedTable: "deleted_bookmarks",
	Path:         "",
}

var Files = FilePath{
	ConfigDir: "",
	DataDir:   "",
	LogDir:    "",
}

var BulletPoint string = "\u2022"
