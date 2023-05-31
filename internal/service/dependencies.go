package service

import rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"

type Database interface {
	LoadSettings() (*rms_notes.NotesSettings, error)
	SaveSettings(val *rms_notes.NotesSettings) error
}
