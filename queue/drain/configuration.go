package drain

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/config"
)

// RedisCredentials changes authentication only. It cannot retarget an
// operation to another host, database, TLS identity or key namespace.
func RedisCredentials(original config.RedisConfiguration, username, password string) (config.RedisConfiguration, error) {
	if original.IsSentinel() {
		return config.RedisConfiguration{}, errors.New("queue fencing does not support Redis Sentinel")
	}
	var location *url.URL
	if strings.TrimSpace(original.Addresses) != "" {
		addresses := original.BuildDsn()
		if len(addresses) != 1 {
			return config.RedisConfiguration{}, errors.New("queue fencing requires a single Redis endpoint")
		}
		var err error
		location, err = url.Parse(addresses[0])
		if err != nil || location.Host == "" {
			return config.RedisConfiguration{}, errors.New("invalid queue Redis endpoint")
		}
	} else {
		location = &url.URL{Scheme: original.Scheme, Host: net.JoinHostPort(original.Host, strconv.Itoa(original.Port))}
		if original.Database != "" {
			location.Path = "/" + original.Database
		}
	}
	if location.Scheme != "redis" && location.Scheme != "rediss" {
		return config.RedisConfiguration{}, errors.New("unsupported queue Redis endpoint scheme")
	}
	location.User = url.UserPassword(username, password)
	result := original
	result.Username, result.Password, result.Addresses = username, password, location.String()
	return result, nil
}

// PostgresExecutorDB uses the same parsed endpoint and TLS settings as the
// application database. Only its authenticated role changes.
func PostgresExecutorDB(cfg config.Configuration) (*sqlx.DB, error) {
	parsed, err := pgx.ParseConfig(cfg.Database.BuildDsn())
	if err != nil {
		return nil, errors.New("invalid queue PostgreSQL endpoint")
	}
	if parsed.User == cfg.Queue.Drain.PostgresExecutorUsername {
		return nil, errors.New("queue executor and normal PostgreSQL roles must differ")
	}
	parsed.User = cfg.Queue.Drain.PostgresExecutorUsername
	parsed.Password = cfg.Queue.Drain.PostgresExecutorPassword
	db := sqlx.NewDb(stdlib.OpenDB(*parsed), "pgx")
	db.SetMaxOpenConns(cfg.Database.EffectiveMaxOpenConnections())
	db.SetMaxIdleConns(cfg.Database.SetMaxIdleConnections)
	return db, nil
}
