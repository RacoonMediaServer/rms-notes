package db

import (
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
)

type notesSettings struct {
	ID       uint                    `gorm:"primaryKey"`
	Settings rms_notes.NotesSettings `gorm:"embedded"`
}

func (d *Database) LoadSettings() (*rms_notes.NotesSettings, error) {
	var record notesSettings
	defaultSettings := notesSettings{
		ID:       1,
		Settings: config.DefaultSettings,
	}
	if err := d.conn.Where(notesSettings{ID: 1}).Attrs(defaultSettings).FirstOrCreate(&record).Error; err != nil {
		return nil, err
	}
	return &record.Settings, nil
}

func (d *Database) SaveSettings(val *rms_notes.NotesSettings) error {
	return d.conn.Save(&notesSettings{ID: 1, Settings: *val}).Error
}
