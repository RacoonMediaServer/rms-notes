package obsidian

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	pathpkg "path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RacoonMediaServer/rms-notes/internal/vault"
	"github.com/studio-b12/gowebdav"
	"go-micro.dev/v4/logger"
)

type note struct {
	modTime time.Time
	tasks   []*Task
}

type Vault struct {
	l          logger.Logger
	vault      vault.Accessor
	baseDir    string
	ctx        context.Context
	sel        atomic.Uint32
	pipeCh     chan deferFn
	errHandler DeferErrHandler
	async      bool

	mu            sync.RWMutex
	notes         map[string]*note
	mapTaskToNote map[string]string
	tasks         map[string]*Task
}

func NewVault(ctx context.Context, directory string, accessor vault.Accessor, fn DeferErrHandler, async bool) *Vault {
	v := &Vault{
		l:             logger.Fields(map[string]interface{}{"from": "obsidian"}),
		vault:         accessor,
		baseDir:       directory,
		ctx:           ctx,
		notes:         map[string]*note{},
		mapTaskToNote: map[string]string{},
		tasks:         map[string]*Task{},
		pipeCh:        make(chan deferFn, pipelineMaxJobs),
		errHandler:    fn,
		async:         async,
	}

	if async {
		go v.processPipeline()
	}
	return v
}

func (v *Vault) modify(kind ErrorKind, fn deferFn, item string) error {
	if v.async {
		v.pipeCh <- wrapDeferFn(kind, fn, item)
		return nil
	}

	return wrapDeferFn(kind, fn, item)()
}

func (v *Vault) GetTasks() []*Task {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var tasks []*Task
	for _, t := range v.tasks {
		copy := *t
		tasks = append(tasks, &copy)
	}
	return tasks
}

func (v *Vault) AddNote(directory, title, content string) error {
	fileName := pathpkg.Join(v.baseDir, directory, escapeFileName(title)+".md")
	return v.modify(ErrAddNoteFailed, func() error {
		return v.vault.Write(fileName, []byte(content))
	}, title)
}

func (v *Vault) AddTask(file string, t *Task) error {
	path := pathpkg.Join(v.baseDir, file)
	return v.modify(ErrAddTaskFailed, func() error {
		tasksFileContent, err := v.vault.Read(path)
		if err != nil {
			if !errors.Is(err, gowebdav.StatusError{Status: 404}) {
				return err
			}
		}

		content := string(tasksFileContent) + "\n" + t.String()
		return v.vault.Write(path, []byte(content))
	}, t.Text)
}

func (v *Vault) SnoozeTask(id string, date time.Time) error {
	v.mu.Lock()
	var t *Task
	note, ok := v.mapTaskToNote[id]
	if ok {
		t, ok = v.tasks[id]
	}
	if !ok {
		v.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}
	t.DueDate = &date
	delete(v.tasks, id)
	delete(v.mapTaskToNote, id)
	v.tasks[t.Hash()] = t
	v.mapTaskToNote[t.Hash()] = note
	v.mu.Unlock()

	return v.modify(ErrSnoozeTaskFailed, func() error {
		lines, err := v.loadNote(note)
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

		return v.saveNote(note, lines)
	}, t.Text)
}

func (v *Vault) RemoveTask(id string) error {
	v.mu.Lock()
	var t *Task
	note, ok := v.mapTaskToNote[id]
	if ok {
		t, ok = v.tasks[id]
	}
	if !ok {
		v.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}
	delete(v.mapTaskToNote, id)
	delete(v.tasks, id)
	v.mu.Unlock()

	return v.modify(ErrRemoveTaskFailed, func() error {
		lines, err := v.loadNote(note)
		if err != nil {
			return err
		}

		idx := -1
		for i, l := range lines {
			if t := ParseTask(l); t != nil && t.Hash() == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("cannot remove task: %s", id)
		}
		lines = append(lines[:idx], lines[idx+1:]...)

		return v.saveNote(note, lines)
	}, t.Text)
}

func (v *Vault) DoneTask(id string) error {
	v.mu.Lock()
	var t *Task
	note, ok := v.mapTaskToNote[id]
	if ok {
		t, ok = v.tasks[id]
	}
	if !ok {
		v.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}
	delete(v.mapTaskToNote, id)
	delete(v.tasks, id)
	v.mu.Unlock()

	return v.modify(ErrDoneTaskFailed, func() error {
		lines, err := v.loadNote(note)
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

		return v.saveNote(note, lines)
	}, t.Text)
}

func (v *Vault) Refresh(selector TaskSelector) error {
	v.l.Log(logger.InfoLevel, "Extracting tasks...")
	defer v.l.Log(logger.InfoLevel, "Extracting DONE")

	mapTaskToNote := make(map[string]string)
	notes := make(map[string]*note)
	tasks := make(map[string]*Task)

	sel := getTaskSelector(selector)

	err := v.vault.Walk(v.baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filterEntry(info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		v.l.Logf(logger.DebugLevel, "Extracting from %s...", path)
		fileTasks, err := v.extractTasks(path, sel)
		if err != nil {
			v.l.Logf(logger.WarnLevel, "Extract tasks from '%s' failed: %s", path, err)
		}
		for _, t := range fileTasks {
			mapTaskToNote[t.Hash()] = path
			tasks[t.Hash()] = t
		}

		n := note{modTime: info.ModTime(), tasks: fileTasks}
		notes[path] = &n

		select {
		case <-v.ctx.Done():
			return v.ctx.Err()
		default:
		}

		return nil
	})

	if err != nil {
		return err
	}

	v.mu.Lock()
	v.mapTaskToNote = mapTaskToNote
	v.notes = notes
	v.tasks = tasks
	v.sel.Store(uint32(selector))
	v.mu.Unlock()

	return nil
}

func (v *Vault) StartWatchingChanges() {
	w := v.vault.Watch(v.baseDir)
	go func() {
		for {
			select {
			case <-w.OnChanged():
				v.handleUpdates()
			case <-v.ctx.Done():
				w.Stop()
				return
			}
		}
	}()
}
