package obsidian

import (
	"context"
	"errors"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	"github.com/RacoonMediaServer/rms-notes/internal/nextcloud"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/go-co-op/gocron"
	"github.com/studio-b12/gowebdav"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
	"path"
	"sync"
	"time"
)

const (
	DateFormat = "2006-01-02"
)

type Manager struct {
	l         logger.Logger
	baseDir   string
	notesDir  string
	tasksFile string
	nc        *nextcloud.Client
	w         *nextcloud.Watcher

	pub micro.Event
	bot rms_bot_client.RmsBotClientService

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	check      chan struct{}
	sched      *gocron.Scheduler
	notifyTime string
	initiated  bool

	mu            sync.RWMutex
	tasks         []*Task
	mapTaskToFile map[string]string
}

func New(settings *rms_notes.NotesSettings, pub micro.Event, bot rms_bot_client.RmsBotClientService) *Manager {
	m := Manager{
		l:          logger.Fields(map[string]interface{}{"from": "obsidian"}),
		baseDir:    settings.Directory,
		notesDir:   path.Join(settings.Directory, settings.NotesDirectory),
		tasksFile:  path.Join(settings.Directory, settings.TasksFile),
		nc:         nextcloud.NewClient(config.Config().WebDAV),
		pub:        pub,
		check:      make(chan struct{}),
		sched:      gocron.NewScheduler(time.Local),
		notifyTime: fmt.Sprintf("%02d:00", settings.NotificationTime),
		bot:        bot,
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.w = m.nc.AddWatcher(m.baseDir)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.observeFolder()
	}()

	_, _ = m.sched.Every(1).Day().At(m.notifyTime).Do(func() {
		m.check <- struct{}{}
	})

	m.sched.StartAsync()
	return &m
}

func (m *Manager) NewNote(title, content string) error {
	fileName := path.Join(m.notesDir, escapeFileName(title)+".md")
	return m.nc.Upload(fileName, []byte(content))
}

func (m *Manager) AddTask(t Task) error {
	tasksFileContent, err := m.nc.Download(m.tasksFile)
	if err != nil {
		if !errors.Is(err, gowebdav.StatusError{Status: 404}) {
			return err
		}
	}

	content := string(tasksFileContent) + "\n" + t.String()
	return m.nc.Upload(m.tasksFile, []byte(content))
}

func (m *Manager) Done(id string) error {
	m.mu.RLock()
	file, ok := m.mapTaskToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	m.mu.RUnlock()

	lines, err := m.loadFile(file)
	if err != nil {
		return err
	}

	now := time.Now()

	for i, l := range lines {
		if t := ParseTask(l); t != nil && t.Hash() == id {
			t.Done = true
			t.DoneDate = &now
			lines[i] = t.String()
			if t.Recurrent != RepetitionNo {
				newLines := make([]string, 0, len(lines)+1)
				next := t.NextDate()
				t.Done = false
				t.DoneDate = nil
				t.DueDate = &next
				newLines = append(newLines, lines[:i]...)
				newLines = append(newLines, t.String())
				newLines = append(newLines, lines[i:]...)
				lines = newLines
			}
			break
		}
	}

	return m.saveFile(file, lines)
}

func (m *Manager) Snooze(id string, date time.Time) error {
	m.mu.RLock()
	file, ok := m.mapTaskToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	m.mu.RUnlock()

	lines, err := m.loadFile(file)
	if err != nil {
		return err
	}

	for i, l := range lines {
		if t := ParseTask(l); t != nil && t.Hash() == id {
			t.DueDate = &date
			lines[i] = t.String()
			break
		}
	}

	return m.saveFile(file, lines)
}

func (m *Manager) Stop() {
	m.sched.Stop()
	m.w.Stop()
	m.cancel()
	m.wg.Wait()
	close(m.check)
}
