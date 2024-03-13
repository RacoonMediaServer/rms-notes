package obsidian

import (
	"context"
	"errors"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/vault"
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

type Vault struct {
	l         logger.Logger
	baseDir   string
	notesDir  string
	tasksFile string
	vault     vault.Accessor
	w         vault.Watcher
	user      int32

	pub micro.Event
	bot rms_bot_client.RmsBotClientService

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	check      chan struct{}
	sched      *gocron.Scheduler
	notifyTime string
	initiated  bool

	mu              sync.RWMutex
	tasks           []*Task
	mapTaskIdToFile map[string]string
	mapTaskIdToTask map[string]*Task
}

func New(settings *rms_notes.NotesSettings, pub micro.Event, bot rms_bot_client.RmsBotClientService, vault vault.Accessor, telegramUser int32) *Vault {
	m := Vault{
		l:               logger.Fields(map[string]interface{}{"from": "obsidian"}),
		baseDir:         settings.Directory,
		notesDir:        path.Join(settings.Directory, settings.NotesDirectory),
		tasksFile:       path.Join(settings.Directory, settings.TasksFile),
		vault:           vault,
		pub:             pub,
		check:           make(chan struct{}),
		sched:           gocron.NewScheduler(time.Local),
		notifyTime:      fmt.Sprintf("%02d:00", settings.NotificationTime),
		bot:             bot,
		user:            telegramUser,
		mapTaskIdToFile: map[string]string{},
		mapTaskIdToTask: map[string]*Task{},
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.w = m.vault.Watch(m.baseDir)

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

func (m *Vault) NewNote(title, content string) error {
	fileName := path.Join(m.notesDir, escapeFileName(title)+".md")
	return m.vault.Write(fileName, []byte(content))
}

func (m *Vault) AddTask(t Task) error {
	tasksFileContent, err := m.vault.Read(m.tasksFile)
	if err != nil {
		if !errors.Is(err, gowebdav.StatusError{Status: 404}) {
			return err
		}
	}

	content := string(tasksFileContent) + "\n" + t.String()
	return m.vault.Write(m.tasksFile, []byte(content))
}

func (m *Vault) Done(id string) error {
	m.mu.RLock()
	file, ok := m.mapTaskIdToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	task := m.mapTaskIdToTask[id]
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

	if err = m.saveFile(file, lines); err != nil {
		return err
	}

	if task != nil {
		m.mu.Lock()
		task.Done = true
		m.mu.Unlock()
	}

	return nil
}

func (m *Vault) Snooze(id string, date time.Time) error {
	m.mu.RLock()
	file, ok := m.mapTaskIdToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	task := m.mapTaskIdToTask[id]
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

	if err = m.saveFile(file, lines); err != nil {
		return err
	}

	if task != nil {
		m.mu.Lock()
		task.DueDate = &date
		m.mu.Unlock()
	}

	return nil
}

func (m *Vault) Stop() {
	m.sched.Stop()
	m.w.Stop()
	m.cancel()
	m.wg.Wait()
	close(m.check)
}
