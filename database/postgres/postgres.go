package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"io"
	"math/rand"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

const pkgName = "postgres"

type DbCtxKey string

const TransactionCtx DbCtxKey = "transaction"

// ErrPendingMigrationsFound is used to indicate there exist pending migrations yet to be run
// if the user proceeds without running migrations it can lead to data integrity issues.
var ErrPendingMigrationsFound = errors.New("migrate: Pending migrations exist, please run convoy migrate first")

type Postgres struct {
	id       int
	dbx      *sqlx.DB
	hook     *hooks.Hook
	pool     *pgxpool.Pool
	replicas []*Postgres
	randGen  *rand.Rand
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	dbConfig := cfg.Database

	primary, err := parseDBConfig(dbConfig)
	primary.id = 0
	replicas := make([]*Postgres, 0)
	for i, replica := range dbConfig.ReadReplicas {
		if replica.Scheme == "" {
			replica.Scheme = dbConfig.Scheme
		}
		r, e := parseDBConfig(replica, "replica ")
		if e != nil {
			return nil, e
		}
		r.id = i + 1
		replicas = append(replicas, r)
	}
	primary.replicas = replicas
	primary.randGen = rand.New(rand.NewSource(time.Now().UnixNano()))

	if err_ := ping(primary); err_ != nil {
		return nil, err_
	}

	return primary, err
}

func parseDBConfig(dbConfig config.DatabaseConfiguration, src ...string) (*Postgres, error) {
	pgxCfg, err := pgxpool.ParseConfig(dbConfig.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("failed to create %sconnection pool: %w", src, err)
	}

	if dbConfig.SetMaxOpenConnections > 0 {
		pgxCfg.MaxConns = int32(dbConfig.SetMaxOpenConnections)
	}
	pgxCfg.MaxConnLifetime = time.Second * time.Duration(dbConfig.SetConnMaxLifetime)
	pgxCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		defer pool.Close()
		return nil, fmt.Errorf("[%s]: failed to open %sdatabase - %v", pkgName, src, err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")

	return &Postgres{dbx: db, pool: pool}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) GetReadDB() *sqlx.DB {
	if len(p.replicas) > 0 {
		r, err := p.getRandomReplica()
		if err != nil || r == nil {
			var id = ""
			if r != nil {
				id = fmt.Sprintf(" %d", r.id)
			}
			log.WithError(err).Errorf("failed to get random replica%s", id)
			return p.dbx
		}
		log.Debugf("fetched replica %d", r.id)
		return r.dbx
	}
	return p.dbx
}

func (p *Postgres) getRandomReplica() (*Postgres, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
		}
	}()
	return p.replicas[p.randGen.Intn(len(p.replicas))], err
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return p.dbx.Close()
}

func (p *Postgres) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return p.dbx.BeginTxx(ctx, nil)
}

func (p *Postgres) Rollback(tx *sqlx.Tx, err error) {
	if err != nil {
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}

	cmErr := tx.Commit()
	if cmErr != nil && !errors.Is(cmErr, sql.ErrTxDone) {
		log.WithError(cmErr).Error("failed to commit tx in ProcessBroadcastEventCreation, rolling back transaction")
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}
}

func (p *Postgres) GetHook() *hooks.Hook {
	if p.hook != nil {
		return p.hook
	}

	hook, err := hooks.Get()
	if err != nil {
		log.Fatal(err)
	}

	p.hook = hook
	return p.hook
}

func (p *Postgres) ReplicaSize() int {
	return len(p.replicas)
}

func (p *Postgres) UnsetReplicas() {
	clear(p.replicas)
}

func ping(p *Postgres) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := p.Ping(ctx)
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(len(p.replicas)+1)*5*time.Second)
	defer cancel()
	err = p.PingReplicas(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.dbx.PingContext(ctx)
}

func (p *Postgres) PingReplicas(ctx context.Context) error {
	for _, replica := range p.replicas {
		if err := replica.dbx.PingContext(ctx); err != nil {
			log.WithError(err).Errorf("replica %d ping failed", replica.id)
			return err
		}
	}
	return nil
}

func GetTx(ctx context.Context, db *sqlx.DB) (*sqlx.Tx, bool, error) {
	isWrapped := false

	wrappedTx, ok := ctx.Value(TransactionCtx).(*sqlx.Tx)
	if ok && wrappedTx != nil {
		isWrapped = true
		return wrappedTx, isWrapped, nil
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, isWrapped, err
	}

	return tx, isWrapped, nil
}

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
}

func closeWithError(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		fmt.Printf("%v, an error occurred while closing the client", err)
	}
}
