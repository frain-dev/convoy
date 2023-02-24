package postgres

import (
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const pkgName = "postgres"

type Postgres struct {
	dbx *sqlx.DB
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	db, err := sqlx.Connect("postgres", cfg.Database.Dsn)
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	db.SetMaxIdleConns(10)                    // The default is defaultMaxIdleConns (= 2)
	db.SetMaxOpenConns(1000)                  // The default is 0 (unlimited)
	db.SetConnMaxLifetime(3600 * time.Second) // The default is 0 (connections reused forever)

	return &Postgres{dbx: db}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

// getPrevPage returns calculated value for the prev page
func getPrevPage(page int) int {
	if page == 0 {
		return 1
	}

	prev := 0
	if page-1 <= 0 {
		prev = page
	} else {
		prev = page - 1
	}

	return prev
}

// getSkip returns calculated skip value for the query
func getSkip(page, limit int) int {
	skip := (page - 1) * limit

	if skip <= 0 {
		skip = 0
	}

	return skip
}

func calculatePaginationData(count, page, perPage int) datastore.PaginationData {
	return datastore.PaginationData{
		Total:     int64(count),
		Page:      int64(page),
		PerPage:   int64(perPage),
		Prev:      int64(getPrevPage(page)),
		Next:      int64(page + 1),
		TotalPage: int64(math.Ceil(float64(count) / float64(perPage))),
	}
}
