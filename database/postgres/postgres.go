package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	log "github.com/frain-dev/convoy/pkg/logger"
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
	conn     *pgxpool.Pool
	hook     *hooks.Hook
	pool     *pgxpool.Pool
	replicas []*Postgres
	randGen  *rand.Rand
	logger   log.Logger
	notices  *noticeSink
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	return NewDBWithLogger(cfg, pkgLogger)
}

func NewDBWithLogger(cfg config.Configuration, logger log.Logger) (*Postgres, error) {
	dbConfig := cfg.Database

	primary, err := parseDBConfig(dbConfig, logger)
	if err != nil {
		return nil, err
	}
	primary.id = 0
	primary.logger = logger
	replicas := make([]*Postgres, 0)

	flag := fflag2.NewFFlag(cfg.EnableFeatureFlag)
	if flag.CanAccessFeature(fflag2.ReadReplicas) {
		for i, replica := range dbConfig.ReadReplicas {
			if replica.Scheme == "" {
				replica.Scheme = dbConfig.Scheme
			}
			r, e := parseDBConfig(replica, logger, "replica ")
			if e != nil {
				return nil, e
			}
			r.id = i + 1
			r.logger = logger
			replicas = append(replicas, r)
		}
	} else if len(dbConfig.ReadReplicas) > 0 {
		logger.Error("read-replicas feature flag required before use")
	}
	primary.replicas = replicas
	primary.randGen = rand.New(rand.NewSource(time.Now().UnixNano()))

	if err_ := ping(primary); err_ != nil {
		return nil, err_
	}

	return primary, err
}

func NewFromConnection(pool *pgxpool.Pool) *Postgres {
	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")
	return &Postgres{dbx: db, pool: pool, conn: pool}
}

// noticeSink lets one caller observe server notices for the duration of an
// operation. Notices are the only progress a long DDL statement emits that
// escapes its transaction, so anything recording progress to a table has to read
// them from here rather than from inside the statement.
type noticeSink struct {
	mu sync.RWMutex
	fn func(*pgconn.Notice)
}

func (s *noticeSink) set(fn func(*pgconn.Notice)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fn = fn
}

func (s *noticeSink) get() func(*pgconn.Notice) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fn
}

// OnNotice routes server notices to fn until it is called again with nil. Only
// one observer is supported, which matches the one-conversion-at-a-time rule the
// partition_runs table enforces.
func (p *Postgres) OnNotice(fn func(*pgconn.Notice)) {
	if p.notices == nil {
		return
	}
	p.notices.set(fn)
}

// noticeHandler forwards server notices to the Convoy log. Partition conversion
// is a single statement that runs for minutes to hours and reports each phase
// with RAISE NOTICE. pgx discards notices unless a handler is installed, so
// without this an operator sees nothing between issuing the command and it
// returning, and nothing at all if the process is killed midway.
func noticeHandler(logger log.Logger, sink *noticeSink) pgconn.NoticeHandler {
	if logger == nil {
		return nil
	}

	return func(_ *pgconn.PgConn, n *pgconn.Notice) {
		if n == nil {
			return
		}

		args := []any{"postgres notice", "severity", n.Severity, "code", n.Code, "message", n.Message}
		if n.Detail != "" {
			args = append(args, "detail", n.Detail)
		}
		if n.Hint != "" {
			args = append(args, "hint", n.Hint)
		}

		// SeverityUnlocalized, because Severity is translated by the server's
		// lc_messages and would not match on a non-English instance. WARNING is
		// a problem signal. The sink feeds partition_runs.phase, so only
		// NOTICE (the conversion's RAISE NOTICE progress) may go there.
		if n.SeverityUnlocalized == "WARNING" {
			logger.Warn(args...)
			return
		}

		if sink != nil {
			if fn := sink.get(); fn != nil {
				fn(n)
			}
		}

		logger.Info(args...)
	}
}

func parseDBConfig(dbConfig config.DatabaseConfiguration, logger log.Logger, src ...string) (*Postgres, error) {
	pgxCfg, err := pgxpool.ParseConfig(dbConfig.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("failed to create %sconnection pool: %w", src, err)
	}

	// Bound the pool even when unset, so a replica cannot open connections until
	// the server refuses them.
	maxConns := dbConfig.EffectiveMaxOpenConnections()
	if dbConfig.SetMaxOpenConnections <= 0 {
		pkgLogger.Warn(fmt.Sprintf("[%s]: SetMaxOpenConnections not set or 0, using default: %d. Set CONVOY_DB_MAX_OPEN_CONN to override.", pkgName, maxConns))
	}
	pgxCfg.MaxConns = int32(maxConns)

	if dbConfig.SetMaxIdleConnections > 0 {
		pgxCfg.MaxConnIdleTime = time.Minute * 5
	}

	pgxCfg.MaxConnLifetime = time.Second * time.Duration(dbConfig.SetConnMaxLifetime)
	pgxCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
	sink := &noticeSink{}
	pgxCfg.ConnConfig.OnNotice = noticeHandler(logger, sink)

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		defer pool.Close()
		return nil, fmt.Errorf("[%s]: failed to open %sdatabase - %v", pkgName, src, err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")

	maxOpenConns := maxConns
	maxIdleConns := dbConfig.SetMaxIdleConnections
	if maxIdleConns <= 0 {
		maxIdleConns = maxOpenConns / 4
		if maxIdleConns < 2 {
			maxIdleConns = 2
		}
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Second * time.Duration(dbConfig.SetConnMaxLifetime))

	return &Postgres{dbx: db, pool: pool, conn: pool, notices: sink}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) GetConn() *pgxpool.Pool {
	return p.conn
}

func (p *Postgres) GetReadDB() *sqlx.DB {
	if len(p.replicas) > 0 {
		r, err := p.getRandomReplica()
		if err != nil || r == nil {
			var id = ""
			if r != nil {
				id = fmt.Sprintf(" %d", r.id)
			}
			p.logger.Error(fmt.Sprintf("failed to get random replica%s: %v", id, err))
			return p.dbx
		}
		p.logger.Debug(fmt.Sprintf("fetched replica %d", r.id))
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
	// Close all replica connections first
	for _, replica := range p.replicas {
		if replica != nil {
			// Close dbx first to return connections to pool
			replica.dbx.Close()
			replica.pool.Close()
		}
	}

	// Close primary connection - close dbx first to return connections to pool
	if p.dbx != nil {
		p.dbx.Close()
	}
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

func (p *Postgres) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return p.dbx.BeginTxx(ctx, nil)
}

func (p *Postgres) Rollback(tx *sqlx.Tx, err error) {
	if err != nil {
		rbErr := tx.Rollback()
		p.logger.Error("failed to roll back transaction in ProcessBroadcastEventCreation", "error", rbErr)
	}

	cmErr := tx.Commit()
	if cmErr != nil && !errors.Is(cmErr, sql.ErrTxDone) {
		p.logger.Error("failed to commit tx in ProcessBroadcastEventCreation, rolling back transaction", "error", cmErr)
		rbErr := tx.Rollback()
		p.logger.Error("failed to roll back transaction in ProcessBroadcastEventCreation", "error", rbErr)
	}
}

func (p *Postgres) GetHook() *hooks.Hook {
	if p.hook != nil {
		return p.hook
	}

	hook, err := hooks.Get()
	if err != nil {
		p.logger.Error(err.Error())
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
			p.logger.Error(fmt.Sprintf("replica %d ping failed: %v", replica.id, err))
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

func closeWithError(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		fmt.Printf("%v, an error occurred while closing the client", err)
	}
}
