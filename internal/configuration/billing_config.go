package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

const loadInstanceBillingConfig = `
SELECT
	id,
	license_key,
	checkout_attempts,
	active_checkout_attempt_id,
	checkout_id,
	external_id,
	license_synced_at,
	created_at,
	updated_at,
	deleted_at
FROM convoy.configurations
WHERE deleted_at IS NULL
LIMIT 1
`

const updateInstanceBillingConfig = `
UPDATE convoy.configurations
SET
	license_key = $1,
	checkout_attempts = $2,
	active_checkout_attempt_id = $3,
	checkout_id = $4,
	external_id = $5,
	license_synced_at = $6,
	updated_at = NOW()
WHERE id = $7 AND deleted_at IS NULL
`

func (s *Service) LoadInstanceBillingConfig(ctx context.Context) (*datastore.Configuration, error) {
	var cfg datastore.Configuration
	var attemptsRaw []byte

	err := s.db.QueryRow(ctx, loadInstanceBillingConfig).Scan(
		&cfg.UID,
		&cfg.LicenseKey,
		&attemptsRaw,
		&cfg.ActiveCheckoutAttemptID,
		&cfg.CheckoutID,
		&cfg.ExternalID,
		&cfg.LicenseSyncedAt,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&cfg.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrConfigNotFound
		}
		s.logger.Error("failed to load instance billing configuration", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	if len(attemptsRaw) > 0 {
		if err := json.Unmarshal(attemptsRaw, &cfg.CheckoutAttempts); err != nil {
			return nil, util.NewServiceError(http.StatusInternalServerError, err)
		}
	}
	if cfg.CheckoutAttempts == nil {
		cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}

	return &cfg, nil
}

func (s *Service) UpdateInstanceBillingConfig(ctx context.Context, cfg *datastore.Configuration) error {
	if cfg == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("configuration cannot be nil"))
	}

	attempts := cfg.CheckoutAttempts
	if attempts == nil {
		attempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
	attemptsRaw, err := json.Marshal(attempts)
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	result, err := s.db.Exec(ctx, updateInstanceBillingConfig,
		cfg.LicenseKey,
		attemptsRaw,
		cfg.ActiveCheckoutAttemptID,
		cfg.CheckoutID,
		cfg.ExternalID,
		nullTimeToInterface(cfg.LicenseSyncedAt),
		cfg.UID,
	)
	if err != nil {
		s.logger.Error("failed to update instance billing configuration", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, errors.New("configuration not found or not updated"))
	}

	return nil
}

func nullTimeToInterface(t null.Time) interface{} {
	if !t.Valid {
		return nil
	}
	return t.Time
}
