package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database represents all database methods
type Database struct {
	conn *gorm.DB
}

func Connect(path string) (*Database, error) {
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		return nil, err
	}
	if err = db.AutoMigrate(&settings{}); err != nil {
		return nil, err
	}
	return &Database{conn: db}, nil
}
