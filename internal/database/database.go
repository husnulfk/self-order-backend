package database

import (
	"selforder/backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a PostgreSQL connection and optionally runs AutoMigrate.
func Connect(dsn string, autoMigrate bool) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	if autoMigrate {
		if err := db.AutoMigrate(
			&models.Table{},
			&models.Category{},
			&models.MenuItem{},
			&models.User{},
			&models.Order{},
			&models.OrderItem{},
			&models.Payment{},
		); err != nil {
			return nil, err
		}
	}

	return db, nil
}
