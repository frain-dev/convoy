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
	checkout_license_key,
	license_key_source,
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
	checkout_license_key = $2,
	license_key_source = $3,
	checkout_attempts = $4,
	active_checkout_attempt_id = $5,
	checkout_id = $6,
	external_id = $7,
	license_synced_at = $8,
	updated_at = NOW()
WHERE id = $9 AND deleted_at IS NULL
`

func (s *Service) LoadInstanceBillingConfig(ctx context.Context) (*datastore.Configuration, error) {
	var cfg datastore.Configuration
	var attemptsRaw []byte

	err := s.db.QueryRow(ctx, loadInstanceBillingConfig).Scan(
		&cfg.UID,
		&cfg.LicenseKey,
		&cfg.CheckoutLicenseKey,
		&cfg.LicenseKeySource,
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

const completeInstanceBillingConfigIfActive = `
UPDATE convoy.configurations
SET
	license_key = $1,
	checkout_license_key = $2,
	license_key_source = $3,
	checkout_attempts = $4,
	active_checkout_attempt_id = $5,
	checkout_id = $6,
	external_id = $7,
	license_synced_at = $8,
	updated_at = NOW()
WHERE id = $9 AND deleted_at IS NULL AND active_checkout_attempt_id = $10
`

// CompleteCheckoutIfActive persists cfg only while the stored
// active_checkout_attempt_id still equals expectedActiveID. It returns
// applied=false (without error) when a concurrent start/complete has already
// superseded or cleared the active attempt, so the caller can treat the
// completion as stale instead of clobbering the newer state. This closes the
// load-then-write race where a superseded attempt could otherwise apply a
// license.
func (s *Service) CompleteCheckoutIfActive(ctx context.Context, cfg *datastore.Configuration, expectedActiveID string) (bool, error) {
	if cfg == nil {
		return false, util.NewServiceError(http.StatusBadRequest, errors.New("configuration cannot be nil"))
	}

	attempts := cfg.CheckoutAttempts
	if attempts == nil {
		attempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
	attemptsRaw, err := json.Marshal(attempts)
	if err != nil {
		return false, util.NewServiceError(http.StatusInternalServerError, err)
	}

	result, err := s.db.Exec(ctx, completeInstanceBillingConfigIfActive,
		cfg.LicenseKey,
		cfg.CheckoutLicenseKey,
		cfg.LicenseKeySource,
		attemptsRaw,
		cfg.ActiveCheckoutAttemptID,
		cfg.CheckoutID,
		cfg.ExternalID,
		nullTimeToInterface(cfg.LicenseSyncedAt),
		cfg.UID,
		expectedActiveID,
	)
	if err != nil {
		s.logger.Error("failed to complete instance billing configuration", "error", err)
		return false, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return result.RowsAffected() > 0, nil
}

const updateInstanceCheckoutAttempts = `
UPDATE convoy.configurations
SET
	checkout_attempts = $1,
	active_checkout_attempt_id = $2,
	checkout_id = $3,
	updated_at = NOW()
WHERE id = $4 AND deleted_at IS NULL
`

// UpdateCheckoutAttempts persists only the checkout-attempt bookkeeping columns
// (attempts map, active attempt id, checkout id). It deliberately never writes
// license_key, external_id, or license_synced_at: those columns are owned by
// checkout completion (CompleteCheckoutIfActive) and the guest-checkout flow.
// Start and supersede paths load the config at request start, so writing the
// license columns from that stale snapshot could erase a license a concurrent
// completion just persisted. Scoping this write to attempt columns makes that
// lost update impossible.
func (s *Service) UpdateCheckoutAttempts(ctx context.Context, cfg *datastore.Configuration) error {
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

	result, err := s.db.Exec(ctx, updateInstanceCheckoutAttempts,
		attemptsRaw,
		cfg.ActiveCheckoutAttemptID,
		cfg.CheckoutID,
		cfg.UID,
	)
	if err != nil {
		s.logger.Error("failed to update instance checkout attempts", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, errors.New("configuration not found or not updated"))
	}

	return nil
}

const completeSupersededCheckout = `
UPDATE convoy.configurations
SET
	checkout_attempts = jsonb_set(checkout_attempts, ARRAY[$1]::text[], $2::jsonb, true),
	checkout_license_key = $3,
	updated_at = NOW()
WHERE id = $4 AND deleted_at IS NULL
`

const completeSupersededCheckoutEffective = `
UPDATE convoy.configurations
SET
	checkout_attempts = jsonb_set(checkout_attempts, ARRAY[$1]::text[], $2::jsonb, true),
	checkout_license_key = $3,
	license_key = $3,
	license_key_source = 'guest_checkout',
	license_synced_at = NOW(),
	updated_at = NOW()
WHERE id = $4 AND deleted_at IS NULL
`

// CompleteSupersededCheckout settles a paid checkout whose attempt was superseded
// by a concurrent start before its CAS completion could land. It marks only this
// attempt's entry completed via jsonb_set (so the concurrent active attempt's
// bookkeeping and the instance-level checkout_id/external_id it now owns are left
// untouched) and records the purchased license. The purchased key always lands in
// checkout_license_key (preserved for reversibility); when makeEffective is true
// (no env license is the active source) it is also promoted to the effective
// license_key with guest_checkout provenance. Persisting the attempt as completed
// is what lets a retry hit the idempotent recovery branch instead of 404 if the
// licenser refresh fails here. Fail closed: an empty key is rejected so a paid
// completion can never wipe the stored key.
func (s *Service) CompleteSupersededCheckout(ctx context.Context, configID, attemptID string, attempt datastore.SelfHostedCheckoutAttempt, purchasedKey string, makeEffective bool) error {
	if purchasedKey == "" {
		return util.NewServiceError(http.StatusBadRequest, errors.New("license key cannot be empty"))
	}
	if attemptID == "" {
		return util.NewServiceError(http.StatusBadRequest, errors.New("attempt id cannot be empty"))
	}

	attemptRaw, err := json.Marshal(attempt)
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	query := completeSupersededCheckout
	if makeEffective {
		query = completeSupersededCheckoutEffective
	}

	result, err := s.db.Exec(ctx, query, attemptID, attemptRaw, purchasedKey, configID)
	if err != nil {
		s.logger.Error("failed to complete superseded checkout", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, errors.New("configuration not found or not updated"))
	}

	return nil
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
		cfg.CheckoutLicenseKey,
		cfg.LicenseKeySource,
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
