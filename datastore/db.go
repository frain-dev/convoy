package datastore

import (
	"errors"

	"github.com/hookcamp/hookcamp/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// New creates a new database connection
func New(cfg config.Configuration) (*gorm.DB, error) {
	var opened gorm.Dialector

	switch cfg.Database.Type {
	case config.PostgresDatabaseProvider:
		opened = postgres.Open(cfg.Database.Dsn)

	case config.MysqlDatabaseProvider:
		opened = mysql.Open(cfg.Database.Dsn)

	default:
		return nil, errors.New("please provide a supported database type")

	}

	db, err := gorm.Open(opened, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}
