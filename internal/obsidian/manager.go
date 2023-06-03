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

	tasks      []*Task
	check      chan struct{}
	sched      *gocron.Scheduler
	notifyTime string
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

func (m *Manager) Stop() {
	m.sched.Stop()
	m.cancel()
	m.wg.Wait()
	close(m.check)
}
