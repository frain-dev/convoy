package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

// Configured registers the deployment's existing connections only. A Redis
// connection used for cache is not a previous queue unless explicitly selected.
// Logical IDs identify registrations, not proven ownership: inspection enables
// no drain actions and cannot establish that another executor is fenced out.
func Configured(cfg config.Configuration, db *sqlx.DB) (*Inventory, error) {
	if err := cfg.ValidateQueueInventory(); err != nil {
		return nil, err
	}
	active := string(cfg.QueueProvider)
	if active == "" {
		active = "redis"
	}
	reader := func(provider string) Reader {
		if provider == "postgres" {
			return Postgres{DB: db}
		}
		return configuredRedis{cfg: cfg.Redis}
	}
	id := func(provider string) string { return StoreID(cfg, provider) }
	registrations := []Registration{{ID: id(active), Provider: active, Role: "active", Reader: reader(active)}}
	if cfg.PreviousQueueProvider != "" {
		previous := string(cfg.PreviousQueueProvider)
		registrations = append(registrations, Registration{ID: id(previous), Provider: previous, Role: "previous", Reader: reader(previous)})
	}
	return New(registrations)
}

// StoreID identifies a configured namespace independently of its credentials.
func StoreID(cfg config.Configuration, provider string) string {
	scope := cfg.QueueStoreScope
	if scope == "" {
		scope = "current"
	}
	digest := sha256.Sum256([]byte(scope + "/" + provider + "/" + configuredLocation(cfg, provider)))
	return hex.EncodeToString(digest[:16])
}

// RedisReader inspects through the supplied authentication without creating a writer.
func RedisReader(cfg config.RedisConfiguration) Reader { return configuredRedis{cfg: cfg} }

type configuredRedis struct{ cfg config.RedisConfiguration }

func (r configuredRedis) withInspector(ctx context.Context, inspect func(*asynq.Inspector) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, err := rdb.NewClientFromRedisConfig(r.cfg)
	if err != nil {
		return err
	}
	// Asynq inspectors use background contexts internally. Give this request
	// its own client with context deadlines enabled and attach the request
	// context to every command, so an unreachable store cannot pin a refresh.
	var bounded redis.UniversalClient
	switch raw := client.Client().(type) {
	case *redis.Client:
		opts := *raw.Options()
		opts.ContextTimeoutEnabled = true
		opts.MaxRetries = -1
		bounded = redis.NewClient(&opts)
	case *redis.ClusterClient:
		opts := *raw.Options()
		opts.ContextTimeoutEnabled = true
		opts.MaxRetries = -1
		bounded = redis.NewClusterClient(&opts)
	default:
		_ = client.Client().Close()
		return errors.New("unsupported inspection connection")
	}
	defer client.Client().Close()
	defer bounded.Close()
	bounded.AddHook(inspectionContext{ctx: ctx})
	// NewInspectorFromRedisClient neither enqueues nor registers any worker.
	inspector := asynq.NewInspectorFromRedisClient(bounded)
	return inspect(inspector)
}

func (r configuredRedis) Inspect(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := r.withInspector(ctx, func(inspector *asynq.Inspector) error {
		var err error
		result, err = (Redis{Inspector: inspector}).Inspect(ctx)
		return err
	})
	return result, err
}

const inspectionTimeout = 5 * time.Second

// Include the configured physical location in the registration identity so a
// changed connection cannot inherit a previous store's cached observation.
// Credentials are excluded; changing a password is not changing stores. DNS
// aliases are not proof of physical identity and remain a drain preflight concern.
func configuredLocation(cfg config.Configuration, provider string) string {
	if provider == "postgres" {
		dc := cfg.Database
		if dsn := dc.BuildDsn(); dsn != "" {
			if parsed, err := pgx.ParseConfig(dsn); err == nil {
				return fmt.Sprintf("%s:%d/%s/convoy", strings.ToLower(parsed.Host), parsed.Port, parsed.Database)
			}
		}
		return fmt.Sprintf("%s:%d/%s/convoy", strings.ToLower(dc.Host), dc.Port, dc.Database)
	}
	addresses := cfg.Redis.SentinelAddresses()
	if !cfg.Redis.IsSentinel() {
		addresses = strings.Split(cfg.Redis.Addresses, ",")
	}
	for i := range addresses {
		address := strings.TrimSpace(addresses[i])
		if parsed, err := url.Parse(address); err == nil && parsed.Host != "" {
			parsed.User = nil
			parsed.Host = strings.ToLower(parsed.Host)
			address = parsed.String()
		}
		addresses[i] = address
	}
	sort.Strings(addresses)
	database := cfg.Redis.Database
	if database == "" {
		database = "0"
	}
	location, _ := json.Marshal([]any{strings.ToLower(cfg.Redis.Host), cfg.Redis.Port, addresses, cfg.Redis.MasterName, database, "asynq"})
	return string(location)
}

type inspectionContext struct{ ctx context.Context }

func (h inspectionContext) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h inspectionContext) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error { return next(h.ctx, cmd) }
}
func (h inspectionContext) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(_ context.Context, cmds []redis.Cmder) error { return next(h.ctx, cmds) }
}
