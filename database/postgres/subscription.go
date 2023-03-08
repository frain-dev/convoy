package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	createSubscription = `
    INSERT INTO convoy.subscriptions (
    id,name,type,
	project_id,endpoint_id,device_id,
	source_id,alert_config_count,alert_config_threshold,
	retry_config_type,retry_config_duration,
	retry_config_retry_count,filter_config_event_types,
	filter_config_filter_headers,filter_config_filter_body,
	rate_limit_config_count,rate_limit_config_duration
	)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17);
    `

	updateSubscription = `
    UPDATE convoy.subscriptions SET
    name=$2,
  	endpoint_id=$3,
 	source_id=$4,
	alert_config_count=$5,
	alert_config_threshold=$6,
	retry_config_type=$7,
	retry_config_duration=$8,
	retry_config_retry_count=$9,
	filter_config_event_types=$10,
	filter_config_filter_headers=$11,
	filter_config_filter_body=$12,
	rate_limit_config_count=$13,
	rate_limit_config_duration=$14
    WHERE id = $1;
    `

	baseFetch = `
    SELECT
    s.id,s.name,s.type,
	s.project_id,
	s.created_at, s.updated_at,

	COALESCE(s.endpoint_id,'') as "endpoint_id",
	COALESCE(s.device_id,'') as "device_id",
	COALESCE(s.source_id,'') as "source_id",

	s.alert_config_count as "alert_config.count",
	s.alert_config_threshold as "alert_config.threshold",
	s.retry_config_type as "retry_config.type",
	s.retry_config_duration as "retry_config.duration",
	s.retry_config_retry_count as "retry_config.retry_count",
	s.filter_config_event_types as "filter_config.event_types",
	s.filter_config_filter_headers as "filter_config.filter.headers",
	s.filter_config_filter_body as "filter_config.filter.body",
	s.rate_limit_config_count as "rate_limit_config.count",
	s.rate_limit_config_duration as "rate_limit_config.duration",

	COALESCE(endpoint_metadata.id,'') as "endpoint_metadata.id",
	COALESCE(endpoint_metadata.title,'') as "endpoint_metadata.title",
	COALESCE(endpoint_metadata.project_id,'') as "endpoint_metadata.project_id",
	COALESCE(endpoint_metadata.support_email,'') as "endpoint_metadata.support_email",
	COALESCE(endpoint_metadata.target_url,'') as "endpoint_metadata.target_url",

	COALESCE(source_metadata.id,'') as "source_metadata.id",
	COALESCE(source_metadata.name,'') as "source_metadata.name",
	COALESCE(source_metadata.type,'') as "source_metadata.type",
	COALESCE(source_metadata.mask_id,'') as "source_metadata.mask_id",
	COALESCE(source_metadata.project_id,'') as "source_metadata.project_id",
 	COALESCE(source_metadata.is_disabled,false) as "source_metadata.is_disabled",
	COALESCE(sv.type, '') as "source_metadata.verifier.type",
	COALESCE(sv.basic_username, '') as "source_metadata.verifier.basic_auth.username",
	COALESCE(sv.basic_password, '') as "source_metadata.verifier.basic_auth.password",
	COALESCE(sv.api_key_header_name, '') as "source_metadata.verifier.api_key.header_name",
	COALESCE(sv.api_key_header_value, '') as "source_metadata.verifier.api_key.header_value",
	COALESCE(sv.hmac_hash, '') as "source_metadata.verifier.hmac.hash",
	COALESCE(sv.hmac_header, '') as "source_metadata.verifier.hmac.header",
	COALESCE(sv.hmac_secret, '') as "source_metadata.verifier.hmac.secret",
	COALESCE(sv.hmac_encoding, '') as "source_metadata.verifier.hmac.encoding"
	FROM convoy.subscriptions s 
	LEFT JOIN convoy.endpoints endpoint_metadata ON s.endpoint_id = endpoint_metadata.id 
	LEFT JOIN convoy.sources source_metadata ON s.source_id = source_metadata.id 
	LEFT JOIN convoy.source_verifiers sv ON sv.id = source_metadata.source_verifier_id 
	WHERE s.deleted_at IS NULL `

	fetchSubscriptionByID = baseFetch + ` AND %s = $1 AND %s = $2;`

	fetchSubscriptionByDeviceID = baseFetch + ` AND %s = $1 AND %s = $2 AND %s = $3`

	fetchCLISubscriptions = baseFetch + `AND %s = $1 AND %s = $2`

	fetchSubscriptionsPaginated = baseFetch + ` AND s.project_id = $1 ORDER BY id desc LIMIT $2 OFFSET $3;`

	fetchSubscriptionsPaginatedFilterByEndpoints = baseFetch + ` AND s.endpoint_id IN (?) AND s.project_id = ? ORDER BY id LIMIT ? OFFSET ?;`

	countSubscriptions = `
	SELECT COUNT(*) FROM convoy.subscriptions WHERE project_id = $1 AND deleted_at IS NULL;
	`

	countSubscriptionsFilterByEndpoints = `
	SELECT COUNT(*) FROM convoy.subscriptions WHERE
	endpoint_id IN (?) AND
	project_id = ? AND
	deleted_at IS NULL;
	`

	deleteSubscriptions = `
	UPDATE convoy.subscriptions SET
	deleted_at = now()
	WHERE id = $1 AND project_id = $2;
	`
)

var (
	ErrSubscriptionNotCreated = errors.New("subscription could not be created")
	ErrSubscriptionNotUpdated = errors.New("subscription could not be updated")
	ErrSubscriptionNotDeleted = errors.New("subscription could not be deleted")
)

type subscriptionRepo struct {
	db *sqlx.DB
}

func NewSubscriptionRepo(db database.Database) datastore.SubscriptionRepository {
	return &subscriptionRepo{db: db.GetDB()}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	var endpointID, sourceID, deviceID *string
	if !util.IsStringEmpty(subscription.EndpointID) {
		endpointID = &subscription.EndpointID
	}

	if !util.IsStringEmpty(subscription.SourceID) {
		sourceID = &subscription.SourceID
	}

	if !util.IsStringEmpty(subscription.DeviceID) {
		deviceID = &subscription.DeviceID
	}

	result, err := s.db.ExecContext(
		ctx, createSubscription, subscription.UID,
		subscription.Name, subscription.Type, subscription.ProjectID,
		endpointID, deviceID, sourceID,
		ac.Count, ac.Threshold, rc.Type, rc.Duration, rc.RetryCount,
		fc.EventTypes, fc.Filter.Headers, fc.Filter.Body, rlc.Count, rlc.Duration,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSubscriptionNotCreated
	}

	return nil
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	result, err := s.db.ExecContext(
		ctx, updateSubscription, subscription.UID,
		subscription.Name, subscription.EndpointID, subscription.SourceID,
		ac.Count, ac.Threshold, rc.Type, rc.Duration, rc.RetryCount,
		fc.EventTypes, fc.Filter.Headers, fc.Filter.Body, rlc.Count, rlc.Duration,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSubscriptionNotUpdated
	}

	return nil
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	var rows *sqlx.Rows
	var err error

	if len(filter.EndpointIDs) > 0 {
		query, args, inerr := sqlx.In(fetchSubscriptionsPaginatedFilterByEndpoints, filter.EndpointIDs, projectID, pageable.Limit(), pageable.Offset())
		if inerr != nil {
			return nil, datastore.PaginationData{}, err
		}
		query = s.db.Rebind(query)

		rows, err = s.db.QueryxContext(ctx, query, args...)
	} else {
		rows, err = s.db.QueryxContext(ctx, fetchSubscriptionsPaginated, projectID, pageable.Limit(), pageable.Offset())
	}

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer rows.Close()

	subscriptions, err := scanSubscriptions(rows)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var count int

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return subscriptions, pagination, err
}

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	result, err := s.db.Exec(deleteSubscriptions, subscription.UID, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSubscriptionNotDeleted
	}

	return nil
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, projectID string, subscriptionID string) (*datastore.Subscription, error) {
	subscription := &datastore.Subscription{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.id", "s.project_id"), subscriptionID, projectID).StructScan(subscription)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}
		return nil, err
	}

	nullifyEmptyConfig(subscription)

	return subscription, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceID(ctx context.Context, projectID string, sourceID string) ([]datastore.Subscription, error) {
	rows, err := s.db.QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.project_id", "s.source_id"), projectID, sourceID)
	if err != nil {
		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) FindSubscriptionsByEndpointID(ctx context.Context, projectId string, endpointID string) ([]datastore.Subscription, error) {
	rows, err := s.db.QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.project_id", "s.endpoint_id"), projectId, endpointID)
	if err != nil {
		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) FindSubscriptionByDeviceID(ctx context.Context, projectId string, deviceID string, subscriptionType datastore.SubscriptionType) (*datastore.Subscription, error) {
	subscription := &datastore.Subscription{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByDeviceID, "s.device_id", "s.project_id", "s.type"), deviceID, projectId, subscriptionType).StructScan(subscription)
	if err != nil {
		return nil, err
	}

	nullifyEmptyConfig(subscription)

	return subscription, nil
}

func (s *subscriptionRepo) FindCLISubscriptions(ctx context.Context, projectID string) ([]datastore.Subscription, error) {
	rows, err := s.db.QueryxContext(ctx, fmt.Sprintf(fetchCLISubscriptions, "s.project_id", "s.type"), projectID, datastore.SubscriptionTypeCLI)
	if err != nil {
		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) TestSubscriptionFilter(ctx context.Context, payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	p, err := flatten.Flatten(payload)
	if err != nil {
		return false, err
	}

	f, err := flatten.Flatten(filter)
	if err != nil {
		return false, err
	}

	isValid := compare.Compare(p, f)

	return isValid, nil
}

var (
	emptyAlertConfig     = datastore.AlertConfiguration{}
	emptyRetryConfig     = datastore.RetryConfiguration{}
	emptyRateLimitConfig = datastore.RateLimitConfiguration{}
)

func nullifyEmptyConfig(sub *datastore.Subscription) {
	if *sub.AlertConfig == emptyAlertConfig {
		sub.AlertConfig = nil
	}

	if *sub.RetryConfig == emptyRetryConfig {
		sub.RetryConfig = nil
	}

	if *sub.RateLimitConfig == emptyRateLimitConfig {
		sub.RateLimitConfig = nil
	}
}

func scanSubscriptions(rows *sqlx.Rows) ([]datastore.Subscription, error) {
	subscriptions := make([]datastore.Subscription, 0)
	var err error

	for rows.Next() {
		sub := datastore.Subscription{}
		err = rows.StructScan(&sub)
		if err != nil {
			return nil, err
		}
		nullifyEmptyConfig(&sub)

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}
