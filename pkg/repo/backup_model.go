package repo

import (
	"log"
	"path/filepath"
	"strconv"

	"github.com/haaag/gm/pkg/util"
)

const _defaultMaxBackups int = 3

type Backup struct {
	Home  string   `json:"path"`
	Files []string `json:"files"`
	Max   int      `json:"max"`
}

func (b *Backup) Load(files []string) {
	b.Files = files
}

func (b *Backup) Last() string {
	if len(b.Files) == 0 {
		return ""
	}
	return b.Files[len(b.Files)-1]
}

func (b *Backup) List() []string {
	return b.Files
}

func (b *Backup) Len() int {
	return len(b.Files)
}

func (b *Backup) GetHome() string {
	return b.Home
}

func (b *Backup) GetMax() int {
	return b.Max
}

func (b *Backup) SetMax(env string) {
	defaultMax := strconv.Itoa(_defaultMaxBackups)
	maxBackups, err := strconv.Atoi(util.GetEnv(env, defaultMax))
	if err != nil {
		log.Fatal(err)
	}
	b.Max = maxBackups
}

func newBackup(path string) *Backup {
	return &Backup{
		Home: filepath.Join(path, "backup"),
		Max:  _defaultMaxBackups,
	}
}
