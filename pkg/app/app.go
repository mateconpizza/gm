// Copyright © 2023 haaag <git.haaag@gmail.com>
package app

import "errors"

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

type App struct {
	Name    string `json:"name"`
	Cmd     string `json:"cmd_name"`
	Version string `json:"version"`
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

type Data struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Tags  string `json:"tags"`
	Desc  string `json:"desc"`
}

type Environment struct {
	Home   string `json:"var_home"`
	Editor string `json:"var_editor"`
}

type TermData struct {
	MaxWidth  int
	MinWidth  int
	MinHeight int
	Color     bool
}

var Config = App{
	Name:    "gomarks",
	Cmd:     "gm",
	Version: "0.0.2",
}

var Info = Data{
	URL:   "https://github.com/haaag/GoMarks#readme",
	Title: "Gomarks",
	Tags:  "golang,awesome,bookmarks",
	Desc:  "Simple yet powerful bookmark manager for your terminal",
}

var Env = Environment{
	Home:   "GOMARKS_HOME",
	Editor: "GOMARKS_EDITOR",
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

var Term = TermData{
	MaxWidth:  100,
	MinWidth:  80,
	MinHeight: 24,
	Color:     true,
}
