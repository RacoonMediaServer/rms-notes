package vault

import (
	"os"
	"path/filepath"
)

type Accessor interface {
	Read(path string) ([]byte, error)
	List(path string) ([]os.FileInfo, error)
	Write(path string, content []byte) error
	Walk(root string, fn filepath.WalkFunc) error
	Watch(path string) Watcher
}

type Watcher interface {
	OnChanged() <-chan struct{}
	Stop()
}
