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

func (m *Manager) collectTasks() error {
	m.l.Log(logger.InfoLevel, "Extracting tasks...")
	defer m.l.Log(logger.InfoLevel, "Extracting DONE")

	var tasks []*Task
	mapTaskToFile := make(map[string]string)

	err := m.nc.Walk(m.baseDir, func(path string, info fs.FileInfo, err error) error {
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
		tasks = append(tasks, fileTasks...)
		for _, t := range tasks {
			mapTaskToFile[t.Hash()] = path
		}
		return nil
	})

	if err != nil {
		return err
	}

	m.mu.Lock()
	m.tasks = tasks
	m.mapTaskToFile = mapTaskToFile
	m.mu.Unlock()

	return nil
}

func (m *Manager) extractTasks(fileName string, selector taskSelector) ([]*Task, error) {
	m.l.Logf(logger.DebugLevel, "Extracting from %s...", fileName)
	var tasks []*Task
	data, err := m.nc.Download(fileName)
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
