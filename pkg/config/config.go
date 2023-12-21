// Copyright © 2023 haaag <git.haaag@gmail.com>
package config

import "errors"

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

type Conf struct {
	App *ConfApp `json:"app"`
	DB  *ConfDB  `json:"database"`
}

type ConfApp struct {
	Name    string          `json:"name"`
	Cmd     string          `json:"command"`
	Version string          `json:"version"`
	Data    ConfData        `json:"data"`
	Env     ConfEnvironment `json:"env"`
	Path    ConfFilePath    `json:"path"`
}

type ConfDB struct {
	Name    string        `json:"name"`
	Table   ConfDBTables  `json:"tables"`
	Type    string        `json:"type"`
	Path    string        `json:"path"`
	Records ConfDBRecords `json:"records"`
}

type ConfDBTables struct {
	Main    string `json:"main"`
	Temp    string `json:"temp"`
	Deleted string `json:"deleted"`
}

type ConfDBRecords struct {
	Main    int `json:"main"`
	Deleted int `json:"deleted"`
}

type ConfFilePath struct {
	Home   string `json:"home"`
	Backup string `json:"backup"`
}

type ConfData struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Tags  string `json:"tags"`
	Desc  string `json:"desc"`
}

type ConfEnvironment struct {
	Home   string `json:"var_home"`
	Editor string `json:"var_editor"`
}

type Terminal struct {
	MaxWidth  int
	MinWidth  int
	MinHeight int
}

var AppConf = Conf{
	App: &App,
	DB:  &DB,
}

var App = ConfApp{
	Name:    "gomarks",
	Cmd:     "gm",
	Version: "0.0.3",
	Data: ConfData{
		URL:   "https://github.com/haaag/GoMarks#readme",
		Title: "Gomarks",
		Tags:  "golang,awesome,bookmarks",
		Desc:  "Simple yet powerful bookmark manager for your terminal",
	},
	Env: ConfEnvironment{
		Home:   "GOMARKS_HOME",
		Editor: "GOMARKS_EDITOR",
	},
	Path: ConfFilePath{
		Home:   "",
		Backup: "",
	},
}

var DB = ConfDB{
	Name: "bookmarks.db",
	Table: ConfDBTables{
		Main:    "bookmarks",
		Temp:    "temp_bookmarks",
		Deleted: "deleted_bookmarks",
	},
	Type: "sqlite",
	Path: "",
}

var Editors = []string{"vim", "nvim", "nano", "emacs", "helix"}

var Term = Terminal{
	MaxWidth:  120,
	MinWidth:  80,
	MinHeight: 20,
}
