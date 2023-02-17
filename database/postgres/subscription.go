package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/frain-dev/convoy/util"

	"github.com/oklog/ulid/v2"

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
    id,name,type,
	project_id,endpoint_id,device_id,source_id,
	alert_config_count as "alert_config.count",
	alert_config_threshold as "alert_config.threshold",
	retry_config_type as "retry_config.type",
	retry_config_duration as "retry_config.duration",
	retry_config_retry_count as "retry_config.retry_count",
	filter_config_event_types as "filter_config.event_types",
	filter_config_filter_headers as "filter_config.filter.headers",
	filter_config_filter_body as "filter_config.filter.body",
	rate_limit_config_count as "rate_limit_config.count",
	rate_limit_config_duration as "rate_limit_config.duration",

	endpoint_metadata.id as "endpoint_metadata.id",
	endpoint_metadata.title as "endpoint_metadata.title",
	endpoint_metadata.project_id as "endpoint_metadata.project_id",
	endpoint_metadata.support_email as "endpoint_metadata.support_email",
	endpoint_metadata.target_url as "endpoint_metadata.target_url",
	endpoint_metadata.secrets as "endpoint_metadata.secrets",

	source_metadata.id as "source_metadata.id",
	source_metadata.name as "source_metadata.name",
	source_metadata.type as "source_metadata.type",
	source_metadata.mask_id as "source_metadata.mask_id",
	source_metadata.project_id as "source_metadata.project_id",
	source_metadata.verifier as "source_metadata.verifier",
	source_metadata.is_disabled as "source_metadata.is_disabled"
	FROM convoy.subscriptions s LEFT JOIN convoy.endpoints endpoint_metadata
    ON s.endpoint_id = endpoint_metadata.id LEFT JOIN convoy.sources source_metadata
    ON s.source_id = source_metadata.id WHERE s.deleted_at IS NULL `

	fetchSubscriptionByID = baseFetch + ` AND %s = $1 AND %s = $2;`

	fetchSubscriptionsPaginated = baseFetch + ` AND s.project_id = $1 ORDER BY id LIMIT $2 OFFSET $3;`

	fetchSubscriptionsPaginatedFilterByEndpoints = baseFetch + ` AND s.endpoint_id IN $1 AND s.project_id = $2 ORDER BY id LIMIT $2 OFFSET $3;`

	countSubscriptions = `
	SELECT COUNT(id) FROM convoy.subscriptions WHERE project_id = $1 AND deleted_at IS NULL;
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

func NewSubscriptionRepo(db *sqlx.DB) datastore.SubscriptionRepository {
	return &subscriptionRepo{db: db}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.UID = ulid.Make().String()
	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	filterHeaders, err := json.Marshal(fc.Filter.Headers)
	if err != nil {
		return err
	}

	filterBody, err := json.Marshal(fc.Filter.Body)
	if err != nil {
		return err
	}

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
		fc.EventTypes, filterHeaders, filterBody, rlc.Count, rlc.Duration,
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

	filterHeaders, err := json.Marshal(fc.Filter.Headers)
	if err != nil {
		return err
	}

	filterBody, err := json.Marshal(fc.Filter.Body)
	if err != nil {
		return err
	}

	result, err := s.db.ExecContext(
		ctx, updateSubscription, subscription.UID,
		subscription.Name, subscription.EndpointID, subscription.SourceID,
		ac.Count, ac.Threshold, rc.Type, rc.Duration, rc.RetryCount,
		fc.EventTypes, filterHeaders, filterBody, rlc.Count, rlc.Duration,
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
	skip := (pageable.Page - 1) * pageable.PerPage
	subscriptions := make([]datastore.Subscription, 0)
	var rows *sqlx.Rows
	var err error

	if len(filter.EndpointIDs) > 0 {
		rows, err = s.db.QueryxContext(ctx, fetchSubscriptionsPaginatedFilterByEndpoints, projectID, filter.EndpointIDs, pageable.PerPage, skip)
	} else {
		rows, err = s.db.QueryxContext(ctx, fetchSubscriptionsPaginated, projectID, pageable.PerPage, skip)
	}

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	sub := datastore.Subscription{}
	for rows.Next() {
		err = rows.StructScan(&sub)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		subscriptions = append(subscriptions, sub)
	}

	var count int
	err = s.db.Get(&count, countSubscriptions, projectID) // TODO: count with filter
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := datastore.PaginationData{
		Total:     int64(count),
		Page:      int64(pageable.Page),
		PerPage:   int64(pageable.PerPage),
		Prev:      int64(getPrevPage(pageable.Page)),
		Next:      int64(pageable.Page + 1),
		TotalPage: int64(math.Ceil(float64(count) / float64(pageable.PerPage))),
	}
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
		return nil, err
	}

	return subscription, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceIDs(ctx context.Context, projectID string, sourceID string) ([]datastore.Subscription, error) {
	subscriptions := make([]datastore.Subscription, 0)
	rows, err := s.db.QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.project_id", "s.source_id"), projectID, sourceID)
	if err != nil {
		return nil, err
	}

	sub := datastore.Subscription{}
	for rows.Next() {
		err = rows.StructScan(&sub)
		if err != nil {
			return nil, err
		}

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsByEndpointID(ctx context.Context, projectId string, endpointID string) ([]datastore.Subscription, error) {
	subscriptions := make([]datastore.Subscription, 0)
	rows, err := s.db.QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.project_id", "s.endpoint_id"), projectId, endpointID)
	if err != nil {
		return nil, err
	}

	sub := datastore.Subscription{}
	for rows.Next() {
		err = rows.StructScan(&sub)
		if err != nil {
			return nil, err
		}

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionByDeviceID(ctx context.Context, projectId string, deviceID string) (*datastore.Subscription, error) {
	subscription := &datastore.Subscription{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, "s.device_id", "s.project_id"), deviceID, projectId).StructScan(subscription)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func (s *subscriptionRepo) TestSubscriptionFilter(ctx context.Context, payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	return true, nil
}
