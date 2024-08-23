package obsidian

import (
	"io/fs"
	"path/filepath"
	"time"

	"go-micro.dev/v4/logger"
)

func (v *Vault) isModified(path string, modTime time.Time) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	note, ok := v.notes[path]
	if !ok {
		return true
	}

	return !note.modTime.Equal(modTime)
}

func (v *Vault) removeNoteUnsafe(path string) {
	note, ok := v.notes[path]
	if !ok {
		return
	}
	for _, t := range note.tasks {
		delete(v.mapTaskToNote, t.Hash())
	}
	delete(v.notes, path)
}

func (v *Vault) addNoteUnsafe(path string, modTime time.Time, tasks []*Task) {
	for _, t := range tasks {
		v.mapTaskToNote[t.Hash()] = path
	}

	n := note{modTime: modTime, tasks: tasks}
	v.notes[path] = &n
}

func (v *Vault) handleUpdates() {
	v.l.Log(logger.InfoLevel, "Updating tasks...")
	defer v.l.Log(logger.InfoLevel, "Updating DONE")

	sel := getTaskSelector(TaskSelector(v.sel.Load()))

	_ = v.vault.Walk(v.baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-v.ctx.Done():
			return v.ctx.Err()
		default:
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

		if !v.isModified(path, info.ModTime()) {
			return nil
		}
		v.l.Logf(logger.InfoLevel, "'%s' is modified, reload", path)
		fileTasks, err := v.extractTasks(path, sel)
		if err != nil {
			v.l.Logf(logger.WarnLevel, "Extract tasks from '%s' failed: %s", path, err)
			return nil
		}

		v.mu.Lock()
		v.removeNoteUnsafe(path)
		v.addNoteUnsafe(path, info.ModTime(), fileTasks)
		v.mu.Unlock()

		return nil
	})
}
