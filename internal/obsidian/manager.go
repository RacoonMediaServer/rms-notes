package obsidian

import (
	"bufio"
	"context"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"go-micro.dev/v4"
	"os"
	"path"
	"sync"
)

const (
	DateFormat = "2006-01-02"
)

type Manager struct {
	baseDir   string
	notesDir  string
	tasksFile string
	pub       micro.Event

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	tasks []*Task
}

func New(settings *rms_notes.NotesSettings, pub micro.Event) *Manager {
	m := Manager{pub: pub}
	m.baseDir = path.Join(config.Config().DataDirectory, settings.Directory)
	m.notesDir = path.Join(m.baseDir, settings.NotesDirectory)
	m.tasksFile = path.Join(m.baseDir, settings.TasksFile)

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.observeFolder()
	}()

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

	content := t.String()

	wr := bufio.NewWriter(f)
	_, err = wr.Write([]byte(content))
	if err != nil {
		return err
	}
	return wr.Flush()
}

func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}
