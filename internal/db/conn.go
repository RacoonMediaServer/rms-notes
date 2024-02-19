package db

import (
	"github.com/RacoonMediaServer/rms-packages/pkg/configuration"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Database represents all database methods
type Database struct {
	conn *gorm.DB
}

func Connect(dbConfig configuration.Database) (*Database, error) {
	db, err := gorm.Open(postgres.Open(dbConfig.GetConnectionString()))
	if err != nil {
		return nil, err
	}
	if err = db.AutoMigrate(&notesSettings{}); err != nil {
		return nil, err
	}
	return &Database{conn: db}, nil
}
