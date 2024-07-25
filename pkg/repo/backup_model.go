package repo

import (
	"fmt"
	"path/filepath"
	"time"
)

const DefBackupMax int = 3

type Backup struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Src  string `json:"source"`
}

func (b *Backup) Fullpath() string {
	return filepath.Join(b.Path, b.Name)
}

func NewBackup(src string) *Backup {
	name := addDatePrefix(filepath.Base(src))

	return &Backup{
		Src:  src,
		Name: name,
		Path: filepath.Join(filepath.Dir(src), "backup"),
	}
}

// addDatePrefix add time.Now() as prefix to a filename.
func addDatePrefix(s string) string {
	now := time.Now().Format(_defBackupDateName)

	return fmt.Sprintf("%s_%s", now, s)
}
