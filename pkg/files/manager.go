package files

import "os"

var DefaultManager = NewFileManager()

type FileManager struct{}

func (fm *FileManager) Exists(path string) bool     { return Exists(path) }
func (fm *FileManager) ExistsErr(path string) error { return ExistsErr(path) }
func (fm *FileManager) Touch(path string, existsOK bool) (*os.File, error) {
	return Touch(path, existsOK)
}
func (fm *FileManager) Find(root, pattern string) ([]string, error) { return Find(root, pattern) }
func (fm *FileManager) Rm(path string) error                        { return Remove(path) }
func (fm *FileManager) RmAll(path string) error                     { return RemoveAll(path) }
func (fm *FileManager) RmEmptyDirs(root string) error               { return RemoveEmptyDirs(root) }
func (fm *FileManager) FindWithExclude(path, fileExt string, exclude ...string) ([]string, error) {
	return ListWithExclude(path, fileExt, exclude...)
}

func NewFileManager() *FileManager {
	return &FileManager{}
}
