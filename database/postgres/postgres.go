package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const pkgName = "postgres"

type Postgres struct {
	dbx    *sqlx.DB
	done   chan bool
	ticker *time.Ticker
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	db, err := sqlx.Connect("postgres", cfg.Database.Dsn)
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	db.SetMaxIdleConns(10)                    // The default is defaultMaxIdleConns (= 2)
	db.SetMaxOpenConns(1000)                  // The default is 0 (unlimited)
	db.SetConnMaxLifetime(3600 * time.Second) // The default is 0 (connections reused forever)

	ticker := time.NewTicker(5 * time.Second)
	done := make(chan bool)
	p := &Postgres{dbx: db, ticker: ticker, done: done}

	// go p.refreshViews()
	return p, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) Close() error {
	close(p.done)
	return p.dbx.Close()
}

func (p *Postgres) refreshViews() {
	done := make(chan bool)
	for {
		select {
		case <-p.ticker.C:
			start := time.Now()
			_, err := p.dbx.Exec(refreshEventMetdataView)
			if err != nil {
				log.WithError(err).Error("failed to refresh event metdata view")
			}

			diff := time.Since(start)
			fmt.Printf("\nDiff is %v\n", diff)

		case <-done:
			p.ticker.Stop()
		}

	}
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

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
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
