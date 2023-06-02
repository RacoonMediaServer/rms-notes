package obsidian

import (
	"context"
	"fmt"
	"github.com/RacoonMediaServer/rms-packages/pkg/events"
	"github.com/RacoonMediaServer/rms-packages/pkg/misc"
	"github.com/radovskyb/watcher"
	"go-micro.dev/v4/logger"
	"path/filepath"
	"regexp"
	"time"
)

const dirMonitoringInterval = 5 * time.Second
const notificationTimeout = 20 * time.Second

func (m *Manager) observeFolder() {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write, watcher.Create, watcher.Move, watcher.Rename, watcher.Remove)

	r := regexp.MustCompile(`\.md$`)
	w.AddFilterHook(watcher.RegexFilterHook(r, true))

	baseDir, err := filepath.EvalSymlinks(m.baseDir)
	if err != nil {
		m.panicMalfunction("Observe obsidian folder failed", err)
	}

	if err = w.AddRecursive(baseDir); err != nil {
		m.panicMalfunction("Observe obsidian folder failed", err)
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer w.Close()
		m.processEvents(w)
	}()

	if err = w.Start(dirMonitoringInterval); err != nil {
		m.panicMalfunction("Observe obsidian folder failed", err)
	}
}

func (m *Manager) processEvents(w *watcher.Watcher) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case e := <-w.Event:
			logger.Infof("[obsidian] %s", e)
			if err := m.collectTasks(); err != nil {
				logger.Warnf("Extract tasks info from Obsidian folder failed: %s", err)
			}
		}
	}
}

func (m *Manager) panicMalfunction(text string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	message := fmt.Sprintf("%s: %s", text, err)
	logger.Error(message)

	event := events.Malfunction{
		Timestamp:  time.Now().Unix(),
		Error:      message,
		System:     events.Malfunction_Services,
		Code:       events.Malfunction_ActionFailed,
		StackTrace: misc.GetStackTrace(),
	}

	if err := m.pub.Publish(ctx, &event); err != nil {
		logger.Warnf("Notify about malfunction failed: %s", err)
	}

	panic(message)
}
