package obsidian

import (
	"bufio"
	"bytes"
	"go-micro.dev/v4/logger"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

type taskSelector func(t *Task) bool

func filterEntry(root string, fi os.FileInfo) bool {
	if fi.IsDir() {
		if fi.Name() == ".obsidian" || fi.Name() == ".trash" {
			return true
		}
		return false
	}

	ext := path.Ext(fi.Name())
	if ext != ".md" {
		return true
	}

	return false
}

func (m *Vault) collectTasks() error {
	m.l.Log(logger.InfoLevel, "Extracting tasks...")
	defer m.l.Log(logger.InfoLevel, "Extracting DONE")

	var tasks []*Task
	mapTaskIdToFile := make(map[string]string)
	mapTaskIdToTask := make(map[string]*Task)

	err := m.vault.Walk(m.baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filterEntry(path, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		fileTasks, err := m.extractTasks(path, selectScheduledTasks)
		if err != nil {
			m.l.Logf(logger.WarnLevel, "Extract tasks from '%s' failed: %s", path, err)
		}
		for _, t := range fileTasks {
			mapTaskIdToFile[t.Hash()] = path
			mapTaskIdToTask[t.Hash()] = t
		}
		tasks = append(tasks, fileTasks...)
		return nil
	})

	if err != nil {
		return err
	}

	m.mu.Lock()
	m.tasks = tasks
	m.mapTaskIdToFile = mapTaskIdToFile
	m.mapTaskIdToTask = mapTaskIdToTask
	m.mu.Unlock()

	return nil
}

func (m *Vault) extractTasks(fileName string, selector taskSelector) ([]*Task, error) {
	m.l.Logf(logger.DebugLevel, "Extracting from %s...", fileName)
	var tasks []*Task
	data, err := m.vault.Read(fileName)
	if err != nil {
		return nil, err
	}
	f := bytes.NewReader(data)

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		if t := ParseTask(scan.Text()); t != nil && selector(t) {
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}

func selectScheduledTasks(t *Task) bool {
	return !t.Done && t.DueDate != nil
}
