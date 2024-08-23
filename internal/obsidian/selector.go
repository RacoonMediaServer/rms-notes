package obsidian

import (
	"os"
	"path"
)

type TaskSelector uint

const (
	All TaskSelector = iota
	Scheduled
)

type taskSelector func(t *Task) bool

func getTaskSelector(s TaskSelector) taskSelector {
	switch s {
	case Scheduled:
		return func(t *Task) bool { return !t.Done && t.DueDate != nil }
	default:
		return func(t *Task) bool { return true }
	}
}

func filterEntry(fi os.FileInfo) bool {
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
