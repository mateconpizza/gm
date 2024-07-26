package app

const (
	name              = "gomarks"
	DefaultDBName     = "bookmarks.db"
	DefaultMaxBackups = 3
	Version           = "0.0.4"
)

var Banner = `┏━╸┏━┓┏┳┓┏━┓┏━┓╻┏ ┏━┓
┃╺┓┃ ┃┃┃┃┣━┫┣┳┛┣┻┓┗━┓
┗━┛┗━┛╹ ╹╹ ╹╹┗╸╹ ╹┗━┛`

type App struct {
	Info    Info   `json:"data"`
	Env     Env    `json:"env"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Banner  string `json:"-"`
	Cmd     string `json:"cmd"`
	Version string `json:"version"`
	Editor  string `json:"editor"`
}

func (a *App) GetName() string {
	return a.Name
}

func (a *App) GetVersion() string {
	return a.Version
}

func (a *App) GetEditorEnv() string {
	return a.Env.Editor
}

type Env struct {
	Home      string `json:"home"`
	Editor    string `json:"editor"`
	BackupMax string `json:"max_backups"`
}

type Info struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Tags  string `json:"tags"`
	Desc  string `json:"desc"`
}

// New creates a new instance of the App struct with default values.
func New() *App {
	return &App{
		Name:    "gomarks",
		Cmd:     "gm",
		Version: Version,
		Banner:  Banner,
		Info: Info{
			URL:   "https://github.com/haaag/GoMarks#readme",
			Title: "Gomarks: A bookmark manager",
			Tags:  "golang,awesome,bookmarks,cli",
			Desc:  "Simple yet powerful bookmark manager for your terminal",
		},
		Env: Env{
			Home:      "GOMARKS_HOME",
			Editor:    "GOMARKS_EDITOR",
			BackupMax: "GOMARKS_BACKUP_MAX",
		},
	}
}
