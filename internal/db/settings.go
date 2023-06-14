package db

import (
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
)

type settings struct {
	ID       uint                    `gorm:"primaryKey"`
	Settings rms_notes.NotesSettings `gorm:"embedded"`
}

func (d *Database) LoadSettings() (*rms_notes.NotesSettings, error) {
	var record settings
	defaultSettings := settings{
		ID:       1,
		Settings: config.DefaultSettings,
	}
	if err := d.conn.Where(settings{ID: 1}).Attrs(defaultSettings).FirstOrCreate(&record).Error; err != nil {
		return nil, err
	}
	return &record.Settings, nil
}

func (d *Database) SaveSettings(val *rms_notes.NotesSettings) error {
	return d.conn.Save(&settings{ID: 1, Settings: *val}).Error
}
