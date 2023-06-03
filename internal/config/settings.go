package config

import rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"

var DefaultSettings = rms_notes.NotesSettings{
	Directory:        "Obsidian",
	NotesDirectory:   "Unsorted",
	TasksFile:        "UnsortedTasks.md",
	NotificationTime: 9,
}
