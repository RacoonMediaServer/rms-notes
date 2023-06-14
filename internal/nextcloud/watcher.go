package nextcloud

import (
	"context"
	"github.com/studio-b12/gowebdav"
	"go-micro.dev/v4/logger"
	"sync"
	"time"
)

const WatchInterval = 5 * time.Second

type Watcher struct {
	c       *gowebdav.Client
	path    string
	ch      chan struct{}
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	modTime time.Time
	l       logger.Logger
}

func (c *Client) AddWatcher(path string) *Watcher {
	w := &Watcher{
		c:    c.c,
		path: path,
		ch:   make(chan struct{}),
		l:    c.l,
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.process()
	}()

	return w
}

func (w *Watcher) OnChanged() <-chan struct{} {
	return w.ch
}

func (w *Watcher) Stop() {
	w.cancel()
	w.wg.Wait()
	close(w.ch)
}

func (w *Watcher) process() {
	w.watch()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-time.After(WatchInterval):
			w.watch()
		}
	}
}

func (w *Watcher) watch() {
	w.l.Log(logger.DebugLevel, "Retrieve changes...")

	stat, err := w.c.Stat(w.path)
	if err != nil {
		w.l.Logf(logger.ErrorLevel, "Retrieve changes failed: %s", err)
		return
	}
	w.l.Log(logger.DebugLevel, "Retrieve DONE.")

	if !stat.ModTime().Equal(w.modTime) {
		w.l.Logf(logger.InfoLevel, "Directory changed, time = %s", stat.ModTime())
		w.modTime = stat.ModTime()
		w.ch <- struct{}{}
	}
}
