package obsidian

import (
	"bufio"
	"context"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/go-co-op/gocron"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
	"os"
	"path"
	"sync"
	"time"
)

const (
	DateFormat = "2006-01-02"
)

type Manager struct {
	baseDir   string
	notesDir  string
	tasksFile string

	pub micro.Event
	bot rms_bot_client.RmsBotClientService

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	check      chan struct{}
	sched      *gocron.Scheduler
	notifyTime string

	mu            sync.RWMutex
	tasks         []*Task
	mapTaskToFile map[string]string
}

func New(settings *rms_notes.NotesSettings, pub micro.Event, bot rms_bot_client.RmsBotClientService) *Manager {
	m := Manager{
		pub:        pub,
		check:      make(chan struct{}),
		sched:      gocron.NewScheduler(time.Local),
		notifyTime: fmt.Sprintf("%02d:00", settings.NotificationTime),
		bot:        bot,
	}
	m.baseDir = path.Join(config.Config().DataDirectory, settings.Directory)
	m.notesDir = path.Join(m.baseDir, settings.NotesDirectory)
	m.tasksFile = path.Join(m.baseDir, settings.TasksFile)

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		if err := m.collectTasks(); err != nil {
			logger.Warnf("Extract tasks info from Obsidian folder failed: %s", err)
		}
		m.checkScheduledTasks()

		m.observeFolder()
	}()

	_, _ = m.sched.Every(1).Day().At(m.notifyTime).Do(func() {
		m.check <- struct{}{}
	})

	m.sched.StartAsync()
	return &m
}

func (m *Manager) checkLayout() error {
	st, err := os.Stat(m.baseDir)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("obsidian directory does not exist")
	}
	return nil
}

func (m *Manager) checkNotesLayout() error {
	if err := m.checkLayout(); err != nil {
		return err
	}

	st, err := os.Stat(m.notesDir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(m.notesDir, 0744); err != nil {
			return fmt.Errorf("create notes directory failed: %w", err)
		}
	} else if !st.IsDir() {
		return fmt.Errorf("notes directory must be a directory")
	}

	return nil
}

func (m *Manager) NewNote(title, content string) error {
	if err := m.checkNotesLayout(); err != nil {
		return err
	}

	fileName := path.Join(m.notesDir, escapeFileName(title)+".md")
	_, err := os.Stat(fileName)
	if !os.IsNotExist(err) {
		return fmt.Errorf("note '%s' already exists", title)
	}
	return os.WriteFile(fileName, []byte(content), 0644)
}

func (m *Manager) AddTask(t Task) error {
	if err := m.checkLayout(); err != nil {
		return err
	}

	f, err := os.OpenFile(m.tasksFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	content := t.String() + "\n"

	wr := bufio.NewWriter(f)
	_, err = wr.Write([]byte(content))
	if err != nil {
		return err
	}
	return wr.Flush()
}

func (m *Manager) Done(id string) error {
	m.mu.RLock()
	file, ok := m.mapTaskToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	m.mu.RUnlock()

	lines, err := loadFile(file)
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

	err = saveFile(file, lines)
	if err == nil {
		err = m.collectTasks()
	}

	return err
}

func (m *Manager) Snooze(id string, date time.Time) error {
	m.mu.RLock()
	file, ok := m.mapTaskToFile[id]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("task not found: %s", id)
	}
	m.mu.RUnlock()

	lines, err := loadFile(file)
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

	err = saveFile(file, lines)
	if err == nil {
		err = m.collectTasks()
	}

	return err
}

func (m *Manager) Stop() {
	m.sched.Stop()
	m.cancel()
	m.wg.Wait()
	close(m.check)
}
