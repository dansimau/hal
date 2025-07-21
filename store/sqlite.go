package store

import (
	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"
)

func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Configure SQLite settings
	if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return nil, err
	}
	if err := db.Exec("PRAGMA synchronous = NORMAL").Error; err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Entity{}); err != nil {
		return nil, err
	}

	return db, nil
}
