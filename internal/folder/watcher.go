package folder

import (
	"time"

	"github.com/RacoonMediaServer/rms-notes/internal/vault"
	"github.com/radovskyb/watcher"
	"go-micro.dev/v4/logger"
)

type folderWatcher struct {
	w  *watcher.Watcher
	ch chan struct{}
}

func newFolderWatcher(path string) vault.Watcher {
	watch := watcher.New()
	ch := make(chan struct{})
	if err := watch.AddRecursive(path); err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case <-watch.Event:
				logger.Logf(logger.InfoLevel, "Something changed")
				ch <- struct{}{}
			case err := <-watch.Error:
				panic(err)
			case <-watch.Closed:
				return
			}
		}
	}()

	go func() {
		if err := watch.Start(1 * time.Second); err != nil {
			panic(err)
		}
	}()

	return &folderWatcher{w: watch, ch: ch}
}

// OnChanged implements vault.Watcher.
func (f *folderWatcher) OnChanged() <-chan struct{} {
	return f.ch
}

// Stop implements vault.Watcher.
func (f *folderWatcher) Stop() {
	f.w.Close()
	close(f.ch)
}
