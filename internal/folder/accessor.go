package folder

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/RacoonMediaServer/rms-notes/internal/vault"
)

type folderAccessor struct {
}

// List implements vault.Accessor.
func (f *folderAccessor) List(path string) ([]fs.FileInfo, error) {
	entries := []fs.FileInfo{}
	err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		entries = append(entries, info)
		return nil
	})
	return entries, err
}

// Read implements vault.Accessor.
func (f *folderAccessor) Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Walk implements vault.Accessor.
func (f *folderAccessor) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

// Write implements vault.Accessor.
func (f *folderAccessor) Write(path string, content []byte) error {
	return os.WriteFile(path, content, 0641)
}

// Watch implements vault.Accessor.
func (f *folderAccessor) Watch(path string) vault.Watcher {
	return newFolderWatcher(path)
}

func NewAccessor() vault.Accessor {
	return &folderAccessor{}
}
