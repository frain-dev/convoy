package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"github.com/frain-dev/convoy/pkg/log"
	"time"
)

type IngestCfg struct {
	db             database.Database
	cache          cache.Cache
	defaultRate    int
	projectID      string
	organisationID string
	cacheTimeout   time.Duration
}

func NewIngestCfg(db database.Database, c cache.Cache, defaultRate int, projectID, organisationID string, cacheTimeoutSeconds int) *IngestCfg {
	cacheTTL := time.Hour
	if cacheTimeoutSeconds > 0 {
		cacheTTL = time.Second * time.Duration(cacheTimeoutSeconds)
	}
	return &IngestCfg{
		db:             db,
		cache:          c,
		defaultRate:    defaultRate,
		projectID:      projectID,
		organisationID: organisationID,
		cacheTimeout:   cacheTTL,
	}
}

func (i *IngestCfg) GetInstanceRateLimitWithCache(ctx context.Context) (int, error) {
	key := instance.KeyInstanceIngestRate

	cacheKey := fmt.Sprintf("rate_limit:%s:%s:%s", key, i.projectID, i.organisationID)

	cachedRate, err := i.getCacheRateLimit(ctx, cacheKey)
	if err == nil && cachedRate > 0 {
		return cachedRate, nil
	}

	rateLimit, err := i.fetchRateLimitFromDatabase(ctx, key, i.projectID, i.organisationID)
	if err != nil {
		return 0, err
	}

	err = i.setCacheRateLimit(ctx, cacheKey, rateLimit)
	if err != nil {
		log.Error("Failed to cache rate limit:", err)
	}

	return rateLimit, nil
}

func (i *IngestCfg) getCacheRateLimit(ctx context.Context, cacheKey string) (int, error) {
	var rateLimit int
	err := i.cache.Get(ctx, cacheKey, &rateLimit)
	if err != nil {
		return 0, err
	}

	return rateLimit, nil
}

func (i *IngestCfg) setCacheRateLimit(ctx context.Context, cacheKey string, rateLimit int) error {
	rateLimitBytes, err := json.Marshal(rateLimit)
	if err != nil {
		return err
	}

	err = i.cache.Set(ctx, cacheKey, rateLimitBytes, i.cacheTimeout)
	if err != nil {
		return err
	}

	return nil
}

func (i *IngestCfg) fetchRateLimitFromDatabase(ctx context.Context, key, projectID, organisationID string) (int, error) {
	var ingestRate instance.IngestRate
	found, err := i.getInstanceOverride(ctx, key, "project", projectID, &ingestRate)
	if err != nil {
		return 0, err
	}
	if !found {
		found, err = i.getInstanceOverride(ctx, key, "organisation", organisationID, &ingestRate)
		if err != nil {
			return 0, err
		}
	}

	// Fallback to default rate if no overrides found
	if !found {
		ingestRate.Value = i.defaultRate
	}

	return ingestRate.Value, nil
}

func (i *IngestCfg) getInstanceOverride(ctx context.Context, key string, scopeType string, scopeId string, model *instance.IngestRate) (bool, error) {
	_, err := instance.FetchDecryptedOverrides(ctx, i.db, key, scopeType, scopeId, &model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return model != nil && model.Value > 0, nil
}
