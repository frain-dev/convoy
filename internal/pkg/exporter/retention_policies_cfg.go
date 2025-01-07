package exporter

import (
	"context"
	"database/sql"
	"errors"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"time"
)

type RetentionCfg struct {
	db             database.Database
	defaultPolicy  config.RetentionPolicyConfiguration
	projectID      string
	organisationID string
}

func NewRetentionCfg(db database.Database, defaultPolicy, projectID, organisationID string) *RetentionCfg {
	return &RetentionCfg{
		db: db,
		defaultPolicy: config.RetentionPolicyConfiguration{
			Policy:                   defaultPolicy,
			IsRetentionPolicyEnabled: false,
		},
		projectID:      projectID,
		organisationID: organisationID,
	}
}

func (r *RetentionCfg) GetRetentionPolicy(ctx context.Context) (time.Duration, error) {
	key := instance.KeyRetentionPolicy

	retentionPolicy, err := r.fetchRetentionPolicyFromDatabase(ctx, key, r.projectID, r.organisationID)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(retentionPolicy)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func (r *RetentionCfg) fetchRetentionPolicyFromDatabase(ctx context.Context, key, projectID, organisationID string) (string, error) {
	var retentionPolicy config.RetentionPolicyConfiguration
	found, err := r.getInstanceOverride(ctx, key, "project", projectID, &retentionPolicy)
	if err != nil {
		return "", err
	}
	if !found {
		found, err = r.getInstanceOverride(ctx, key, "organisation", organisationID, &retentionPolicy)
		if err != nil {
			return "", err
		}
	}

	if !found {
		found, err = r.getInstanceDefault(ctx, key, "project", &retentionPolicy)
		if err != nil {
			return "", err
		}
		if !found {
			found, err = r.getInstanceDefault(ctx, key, "organisation", &retentionPolicy)
			if err != nil {
				return "", err
			}
		}
	}

	// If no value found, fallback to default configuration
	if !found {
		retentionPolicy = r.defaultPolicy
	}

	return retentionPolicy.Policy, nil
}

func (r *RetentionCfg) getInstanceOverride(ctx context.Context, key, scopeType, scopeID string, model *config.RetentionPolicyConfiguration) (bool, error) {
	_, err := instance.FetchDecryptedOverrides(ctx, r.db, key, scopeType, scopeID, &model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return model != nil && model.Policy != "", nil
}

func (r *RetentionCfg) getInstanceDefault(ctx context.Context, key, scopeType string, model *config.RetentionPolicyConfiguration) (bool, error) {
	_, err := instance.FetchDecryptedDefaults(ctx, r.db, key, scopeType, &model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return model != nil && model.Policy != "", nil
}
