package service

import (
	"github.com/RacoonMediaServer/rms-notes/internal/model"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
)

type Database interface {
	LoadSettings() (*rms_notes.NotesSettings, error)
	SaveSettings(val *rms_notes.NotesSettings) error

	LoadUsers() (map[int32]*model.NotesUser, error)
	AddUser(camera *model.NotesUser) error
}
