package obsidian

import (
	"bufio"
	"go-micro.dev/v4/logger"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

type taskSelector func(t *Task) bool

func (m *Manager) collectTasks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks = nil
	m.mapTaskToFile = make(map[string]string)
	baseDir, err := filepath.EvalSymlinks(m.baseDir)
	if err != nil {
		return err
	}

	return filepath.Walk(baseDir, func(fileName string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		ext := path.Ext(fileName)
		if ext != ".md" {
			return nil
		}

		if isFileHidden(fileName) {
			return nil
		}
		tasks, err := extractTasks(fileName, selectScheduledTasks)
		if err != nil {
			logger.Warnf("Extract tasks from '%s' failed: %s", fileName, err)
		}
		m.tasks = append(m.tasks, tasks...)
		for _, t := range tasks {
			m.mapTaskToFile[t.Hash()] = fileName
		}

		return nil
	})
}

func extractTasks(fileName string, selector taskSelector) ([]*Task, error) {
	var tasks []*Task
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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
