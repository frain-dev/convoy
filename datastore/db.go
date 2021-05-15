package datastore

import (
	"errors"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type database struct {
	inner   *gorm.DB
	dialect config.DatabaseProvider
}

func (db *database) Close() error {

	d, err := db.inner.DB()
	if err != nil {
		return err
	}

	return d.Close()
}

func (db *database) Migrate() error {
	return db.inner.AutoMigrate(hookcamp.Organisation{},
		hookcamp.Application{},
		hookcamp.Endpoint{})
}

// New creates a new database connection
func New(cfg config.Configuration) (hookcamp.Datastore, error) {

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

	return &database{
		inner:   db,
		dialect: cfg.Database.Type,
	}, nil
}
