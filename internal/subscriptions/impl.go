package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/subscriptions/repo"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrSubscriptionNotCreated = errors.New("subscription could not be created")
	ErrSubscriptionNotUpdated = errors.New("subscription could not be updated")
	ErrSubscriptionNotDeleted = errors.New("subscription could not be deleted")
)

// ServiceError represents an error that occurs during service operations
type ServiceError struct {
	ErrMsg string
	Err    error
}

func (s *ServiceError) Error() string {
	return s.ErrMsg
}

func (s *ServiceError) Unwrap() error {
	return s.Err
}

// Service implements the SubscriptionRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.SubscriptionRepository at compile time
var _ datastore.SubscriptionRepository = (*Service)(nil)

// New creates a new Subscription Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// ============================================================================
// CREATE Operations
// ============================================================================

func (s *Service) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	// Flatten filter configs
	err := fc.Filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = fc.Filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	fc.Filter.IsFlattened = true

	// Begin transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return &ServiceError{ErrMsg: "failed to create subscription", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Prepare parameters
	alertCount, alertThreshold := alertConfigToParams(&ac)
	retryType, retryDuration, retryRetryCount := retryConfigToParams(&rc)
	filterParams := filterConfigToParams(&fc)
	rateLimitCount, rateLimitDuration := rateLimitConfigToParams(&rlc)

	// Create subscription
	err = qtx.CreateSubscription(ctx, repo.CreateSubscriptionParams{
		ID:                            subscription.UID,
		Name:                          subscription.Name,
		Type:                          string(subscription.Type),
		ProjectID:                     subscription.ProjectID,
		EndpointID:                    stringToPgText(subscription.EndpointID),
		DeviceID:                      stringToPgText(subscription.DeviceID),
		SourceID:                      stringToPgText(subscription.SourceID),
		AlertConfigCount:              alertCount,
		AlertConfigThreshold:          alertThreshold,
		RetryConfigType:               retryType,
		RetryConfigDuration:           retryDuration,
		RetryConfigRetryCount:         retryRetryCount,
		FilterConfigEventTypes:        filterParams.eventTypes,
		FilterConfigFilterHeaders:     filterParams.headers,
		FilterConfigFilterBody:        filterParams.body,
		FilterConfigFilterIsFlattened: filterParams.isFlattened,
		FilterConfigFilterRawHeaders:  filterParams.rawHeaders,
		FilterConfigFilterRawBody:     filterParams.rawBody,
		RateLimitConfigCount:          rateLimitCount,
		RateLimitConfigDuration:       rateLimitDuration,
		Function:                      stringToPgText(subscription.Function.String),
		DeliveryMode:                  stringToPgText(string(subscription.DeliveryMode)),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create subscription")
		return &ServiceError{ErrMsg: "failed to create subscription", Err: err}
	}

	// Update subscription with raw filters for use in event type insertion
	subscription.FilterConfig.Filter.Headers = subscription.FilterConfig.Filter.RawHeaders
	subscription.FilterConfig.Filter.Body = subscription.FilterConfig.Filter.RawBody

	// Create event types for each subscription
	eventTypesSlice := make([]repo.UpsertSubscriptionEventTypesParams, len(subscription.FilterConfig.EventTypes))
	for i := range subscription.FilterConfig.EventTypes {
		eventTypesSlice[i] = repo.UpsertSubscriptionEventTypesParams{
			ID:          ulid.Make().String(),
			Name:        subscription.FilterConfig.EventTypes[i],
			ProjectID:   subscription.ProjectID,
			Description: pgtype.Text{String: "", Valid: false},
			Category:    pgtype.Text{String: "", Valid: false},
		}
	}

	// Batch insert event types
	for _, et := range eventTypesSlice {
		err = qtx.UpsertSubscriptionEventTypes(ctx, et)
		if err != nil {
			s.logger.WithError(err).Error("failed to upsert event types")
			return &ServiceError{ErrMsg: "failed to create subscription event types", Err: err}
		}
	}

	// Create filters for each event type
	err = qtx.InsertSubscriptionEventTypeFilters(ctx, stringToPgText(subscription.UID))
	if err != nil {
		s.logger.WithError(err).Error("failed to insert event type filters")
		return &ServiceError{ErrMsg: "failed to create subscription filters", Err: err}
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return &ServiceError{ErrMsg: "failed to create subscription", Err: err}
	}

	return nil
}

// ============================================================================
// UPDATE Operations
// ============================================================================

func (s *Service) UpdateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	// Flatten filter configs
	err := fc.Filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = fc.Filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	fc.Filter.IsFlattened = true

	// Begin transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return &ServiceError{ErrMsg: "failed to update subscription", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Prepare parameters
	alertCount, alertThreshold := alertConfigToParams(&ac)
	retryType, retryDuration, retryRetryCount := retryConfigToParams(&rc)
	filterParams := filterConfigToParams(&fc)
	rateLimitCount, rateLimitDuration := rateLimitConfigToParams(&rlc)

	// Update subscription
	result, err := qtx.UpdateSubscription(ctx, repo.UpdateSubscriptionParams{
		ID:                            subscription.UID,
		ProjectID:                     projectID,
		Name:                          subscription.Name,
		EndpointID:                    stringToPgText(subscription.EndpointID),
		SourceID:                      stringToPgText(subscription.SourceID),
		AlertConfigCount:              alertCount,
		AlertConfigThreshold:          alertThreshold,
		RetryConfigType:               retryType,
		RetryConfigDuration:           retryDuration,
		RetryConfigRetryCount:         retryRetryCount,
		FilterConfigEventTypes:        filterParams.eventTypes,
		FilterConfigFilterHeaders:     filterParams.headers,
		FilterConfigFilterBody:        filterParams.body,
		FilterConfigFilterIsFlattened: filterParams.isFlattened,
		FilterConfigFilterRawHeaders:  filterParams.rawHeaders,
		FilterConfigFilterRawBody:     filterParams.rawBody,
		RateLimitConfigCount:          rateLimitCount,
		RateLimitConfigDuration:       rateLimitDuration,
		Function:                      stringToPgText(subscription.Function.String),
		DeliveryMode:                  stringToPgText(string(subscription.DeliveryMode)),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update subscription")
		return &ServiceError{ErrMsg: "failed to update subscription", Err: err}
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected < 1 {
		return datastore.ErrSubscriptionNotFound
	}

	// Update subscription with raw filters
	subscription.FilterConfig.Filter.Headers = subscription.FilterConfig.Filter.RawHeaders
	subscription.FilterConfig.Filter.Body = subscription.FilterConfig.Filter.RawBody

	// Create event types for each subscription
	eventTypesSlice := make([]repo.UpsertSubscriptionEventTypesParams, len(subscription.FilterConfig.EventTypes))
	for i := range subscription.FilterConfig.EventTypes {
		eventTypesSlice[i] = repo.UpsertSubscriptionEventTypesParams{
			ID:          ulid.Make().String(),
			Name:        subscription.FilterConfig.EventTypes[i],
			ProjectID:   subscription.ProjectID,
			Description: pgtype.Text{String: "", Valid: false},
			Category:    pgtype.Text{String: "", Valid: false},
		}
	}

	// Batch insert event types
	for _, et := range eventTypesSlice {
		err = qtx.UpsertSubscriptionEventTypes(ctx, et)
		if err != nil {
			s.logger.WithError(err).Error("failed to upsert event types")
			return &ServiceError{ErrMsg: "failed to update subscription event types", Err: err}
		}
	}

	// Delete filters when they are removed from the subscription
	err = qtx.DeleteSubscriptionEventTypes(ctx, subscription.UID)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete old event type filters")
		return &ServiceError{ErrMsg: "failed to delete old subscription filters", Err: err}
	}

	// Create filters for each event type
	err = qtx.InsertSubscriptionEventTypeFilters(ctx, stringToPgText(subscription.UID))
	if err != nil {
		s.logger.WithError(err).Error("failed to insert event type filters")
		return &ServiceError{ErrMsg: "failed to create subscription filters", Err: err}
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return &ServiceError{ErrMsg: "failed to update subscription", Err: err}
	}

	return nil
}

// ============================================================================
// FETCH Operations
// ============================================================================

func (s *Service) FindSubscriptionByID(ctx context.Context, projectID, subscriptionID string) (*datastore.Subscription, error) {
	row, err := s.repo.FetchSubscriptionByID(ctx, repo.FetchSubscriptionByIDParams{
		ID:        subscriptionID,
		ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}
		s.logger.WithError(err).Error("failed to fetch subscription by ID")
		return nil, &ServiceError{ErrMsg: "failed to fetch subscription", Err: err}
	}

	return rowToSubscription(row)
}

func (s *Service) FindSubscriptionsBySourceID(ctx context.Context, projectID, sourceID string) ([]datastore.Subscription, error) {
	rows, err := s.repo.FetchSubscriptionsBySourceID(ctx, repo.FetchSubscriptionsBySourceIDParams{
		ProjectID: projectID,
		SourceID:  stringToPgText(sourceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}
		s.logger.WithError(err).Error("failed to fetch subscriptions by source ID")
		return nil, &ServiceError{ErrMsg: "failed to fetch subscriptions", Err: err}
	}

	subscriptions := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := rowToSubscription(row)
		if err != nil {
			continue
		}
		if sub != nil {
			subscriptions = append(subscriptions, *sub)
		}
	}

	return subscriptions, nil
}

func (s *Service) FindSubscriptionsByEndpointID(ctx context.Context, projectID, endpointID string) ([]datastore.Subscription, error) {
	rows, err := s.repo.FetchSubscriptionsByEndpointID(ctx, repo.FetchSubscriptionsByEndpointIDParams{
		ProjectID:  projectID,
		EndpointID: stringToPgText(endpointID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}
		s.logger.WithError(err).Error("failed to fetch subscriptions by endpoint ID")
		return nil, &ServiceError{ErrMsg: "failed to fetch subscriptions", Err: err}
	}

	subscriptions := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := rowToSubscription(row)
		if err != nil {
			continue
		}
		if sub != nil {
			subscriptions = append(subscriptions, *sub)
		}
	}

	return subscriptions, nil
}

func (s *Service) FindSubscriptionByDeviceID(ctx context.Context, projectID, deviceID string, subscriptionType datastore.SubscriptionType) (*datastore.Subscription, error) {
	row, err := s.repo.FetchSubscriptionByDeviceID(ctx, repo.FetchSubscriptionByDeviceIDParams{
		DeviceID:         stringToPgText(deviceID),
		ProjectID:        projectID,
		SubscriptionType: string(subscriptionType),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}
		s.logger.WithError(err).Error("failed to fetch subscription by device ID")
		return nil, &ServiceError{ErrMsg: "failed to fetch subscription", Err: err}
	}

	return rowToSubscription(row)
}

func (s *Service) FindCLISubscriptions(ctx context.Context, projectID string) ([]datastore.Subscription, error) {
	rows, err := s.repo.FetchCLISubscriptions(ctx, projectID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch CLI subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to fetch CLI subscriptions", Err: err}
	}

	subscriptions := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := rowToSubscription(row)
		if err != nil {
			continue
		}
		if sub != nil {
			subscriptions = append(subscriptions, *sub)
		}
	}

	return subscriptions, nil
}

// ============================================================================
// PAGINATED Operations
// ============================================================================

func (s *Service) LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	var direction string
	if pageable.Direction == datastore.Next {
		direction = "next"
	} else {
		direction = "prev"
	}

	// Prepare filters
	hasEndpointFilter := len(filter.EndpointIDs) > 0
	endpointIDs := filter.EndpointIDs
	if !hasEndpointFilter {
		endpointIDs = []string{}
	}

	hasNameFilter := !util.IsStringEmpty(filter.SubscriptionName)
	nameFilter := ""
	if hasNameFilter {
		nameFilter = fmt.Sprintf("%%%s%%", filter.SubscriptionName)
	}

	// Fetch subscriptions
	rows, err := s.repo.FetchSubscriptionsPaginated(ctx, repo.FetchSubscriptionsPaginatedParams{
		ProjectID:         projectID,
		Direction:         direction,
		Cursor:            pageable.Cursor(),
		HasEndpointFilter: hasEndpointFilter,
		EndpointIds:       endpointIDs,
		HasNameFilter:     hasNameFilter,
		NameFilter:        nameFilter,
		LimitVal:          int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch subscriptions paginated")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "failed to fetch subscriptions", Err: err}
	}

	subscriptions := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := rowToSubscription(row)
		if err != nil {
			continue
		}
		if sub != nil {
			subscriptions = append(subscriptions, *sub)
		}
	}

	var prevRowCount datastore.PrevRowCount
	if len(subscriptions) > 0 {
		first := subscriptions[0]

		count, err := s.repo.CountPrevSubscriptions(ctx, repo.CountPrevSubscriptionsParams{
			ProjectID:         projectID,
			Cursor:            first.UID,
			HasEndpointFilter: hasEndpointFilter,
			EndpointIds:       endpointIDs,
			HasNameFilter:     hasNameFilter,
			NameFilter:        nameFilter,
		})
		if err == nil {
			prevRowCount = datastore.PrevRowCount{Count: int(count)}
		}
	}

	ids := make([]string, len(subscriptions))
	for i := range subscriptions {
		ids[i] = subscriptions[i].UID
	}

	if len(subscriptions) > pageable.PerPage {
		subscriptions = subscriptions[:len(subscriptions)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return subscriptions, *pagination, nil
}

// ============================================================================
// DELETE Operations
// ============================================================================

func (s *Service) DeleteSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	result, err := s.repo.DeleteSubscription(ctx, repo.DeleteSubscriptionParams{
		ID:        subscription.UID,
		ProjectID: projectID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to delete subscription")
		return &ServiceError{ErrMsg: "failed to delete subscription", Err: err}
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected < 1 {
		return datastore.ErrSubscriptionNotFound
	}

	return nil
}

// ============================================================================
// BROADCAST & SYNC Operations
// ============================================================================

func (s *Service) FetchSubscriptionsForBroadcast(ctx context.Context, projectID, eventType string, pageSize int) ([]datastore.Subscription, error) {
	var allSubs []datastore.Subscription
	cursor := "0"

	for {
		rows, err := s.repo.FetchSubscriptionsForBroadcast(ctx, repo.FetchSubscriptionsForBroadcastParams{
			EventType: eventType,
			Cursor:    cursor,
			ProjectID: projectID,
			LimitVal:  int64(pageSize),
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to fetch subscriptions for broadcast")
			return nil, &ServiceError{ErrMsg: "failed to fetch subscriptions for broadcast", Err: err}
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			sub := &datastore.Subscription{
				UID:        row.ID,
				Type:       datastore.SubscriptionType(row.Type),
				ProjectID:  row.ProjectID,
				EndpointID: pgTextToString(row.EndpointID),
				Function:   null.NewString(pgTextToString(row.Function), row.Function.Valid),
				FilterConfig: paramsToFilterConfig(
					row.FilterConfigEventTypes,
					row.FilterConfigFilterHeaders,
					row.FilterConfigFilterBody,
					row.FilterConfigFilterIsFlattened,
					[]byte("{}"),
					[]byte("{}"),
				),
			}
			allSubs = append(allSubs, *sub)
		}

		cursor = rows[len(rows)-1].ID
	}

	return allSubs, nil
}

func (s *Service) LoadAllSubscriptionConfig(ctx context.Context, projectIDs []string, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	// Count total subscriptions
	totalCount, err := s.repo.CountProjectSubscriptions(ctx, projectIDs)
	if err != nil {
		s.logger.WithError(err).Error("failed to count subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to count subscriptions", Err: err}
	}

	if totalCount == 0 {
		return []datastore.Subscription{}, nil
	}

	subs := make([]datastore.Subscription, 0, totalCount)
	cursor := "0"
	numBatches := int64(math.Ceil(float64(totalCount) / float64(pageSize)))

	for i := int64(0); i < numBatches; i++ {
		rows, err := s.repo.LoadAllSubscriptionsConfiguration(ctx, repo.LoadAllSubscriptionsConfigurationParams{
			Cursor:     cursor,
			ProjectIds: projectIDs,
			LimitVal:   int64(pageSize),
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to load subscriptions config")
			return nil, &ServiceError{ErrMsg: "failed to load subscriptions config", Err: err}
		}

		for _, row := range rows {
			sub := &datastore.Subscription{
				Name:       row.Name,
				UID:        row.ID,
				Type:       datastore.SubscriptionType(row.Type),
				ProjectID:  row.ProjectID,
				EndpointID: pgTextToString(row.EndpointID),
				Function:   null.NewString(pgTextToString(row.Function), row.Function.Valid),
				UpdatedAt:  pgTimestamptzToTime(row.UpdatedAt),
				FilterConfig: paramsToFilterConfig(
					row.FilterConfigEventTypes,
					row.FilterConfigFilterHeaders,
					row.FilterConfigFilterBody,
					row.FilterConfigFilterIsFlattened,
					[]byte("{}"),
					[]byte("{}"),
				),
			}
			subs = append(subs, *sub)
			cursor = row.ID
		}
	}

	return subs, nil
}

func (s *Service) FetchDeletedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 || len(subscriptionUpdates) == 0 {
		return []datastore.Subscription{}, nil
	}

	ids := make([]string, 0, len(subscriptionUpdates))
	for _, sub := range subscriptionUpdates {
		ids = append(ids, sub.UID)
	}

	rows, err := s.repo.FetchDeletedSubscriptions(ctx, repo.FetchDeletedSubscriptionsParams{
		SubscriptionIds: ids,
		ProjectIds:      projectIDs,
		LimitVal:        int64(pageSize),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch deleted subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to fetch deleted subscriptions", Err: err}
	}

	subs := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub := &datastore.Subscription{
			UID:       row.ID,
			ProjectID: row.ProjectID,
			DeletedAt: null.NewTime(pgTimestamptzToTime(row.DeletedAt), row.DeletedAt.Valid),
			FilterConfig: &datastore.FilterConfiguration{
				EventTypes: row.FilterConfigEventTypes,
			},
		}
		subs = append(subs, *sub)
	}

	return subs, nil
}

func (s *Service) FetchUpdatedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 || len(subscriptionUpdates) == 0 {
		return []datastore.Subscription{}, nil
	}

	// Build dynamic VALUES clause for input_map CTE
	valuesSQL := ""
	args := []interface{}{}
	argIdx := 1

	for i, update := range subscriptionUpdates {
		valuesSQL += fmt.Sprintf("($%d, $%d::timestamptz)", argIdx, argIdx+1)
		args = append(args, update.UID, update.UpdatedAt)
		argIdx += 2

		if i < len(subscriptionUpdates)-1 {
			valuesSQL += ", "
		}
	}

	// Add project IDs and limit
	args = append(args, projectIDs, pageSize)

	// Build the complete query
	query := fmt.Sprintf(`
WITH input_map(id, last_updated_at) AS (
    VALUES %s
),
updated_existing AS (
    SELECT
        s.name,
        s.id,
        s.type,
        s.project_id,
        s.endpoint_id,
        s.device_id,
        s.source_id,
        s.function,
        s.delivery_mode,
        s.updated_at,
        s.created_at,
        s.alert_config_count,
        s.alert_config_threshold,
        s.retry_config_type,
        s.retry_config_duration,
        s.retry_config_retry_count,
        s.filter_config_event_types,
        s.filter_config_filter_headers,
        s.filter_config_filter_body,
        s.filter_config_filter_is_flattened,
        s.filter_config_filter_raw_headers,
        s.filter_config_filter_raw_body,
        s.rate_limit_config_count,
        s.rate_limit_config_duration
    FROM convoy.subscriptions s
    JOIN input_map m ON s.id = m.id
    WHERE s.updated_at > m.last_updated_at
        AND s.project_id = ANY($%d::text[])
        AND s.deleted_at IS NULL
),
new_subscriptions AS (
    SELECT
        s.name,
        s.id,
        s.type,
        s.project_id,
        s.endpoint_id,
        s.device_id,
        s.source_id,
        s.function,
        s.delivery_mode,
        s.updated_at,
        s.created_at,
        s.alert_config_count,
        s.alert_config_threshold,
        s.retry_config_type,
        s.retry_config_duration,
        s.retry_config_retry_count,
        s.filter_config_event_types,
        s.filter_config_filter_headers,
        s.filter_config_filter_body,
        s.filter_config_filter_is_flattened,
        s.filter_config_filter_raw_headers,
        s.filter_config_filter_raw_body,
        s.rate_limit_config_count,
        s.rate_limit_config_duration
    FROM convoy.subscriptions s
    WHERE s.id NOT IN (SELECT id FROM input_map)
        AND s.project_id = ANY($%d::text[])
        AND s.deleted_at IS NULL
)
SELECT * FROM updated_existing
UNION ALL
SELECT * FROM new_subscriptions
ORDER BY id
LIMIT $%d`, valuesSQL, argIdx, argIdx, argIdx+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch updated subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to fetch updated subscriptions", Err: err}
	}
	defer rows.Close()

	subs := make([]datastore.Subscription, 0)
	for rows.Next() {
		var name, id, subType, projectID string
		var endpointID, deviceID, sourceID, function, deliveryMode pgtype.Text
		var updatedAt, createdAt pgtype.Timestamptz
		var alertCount int32
		var alertThreshold string
		var retryType string
		var retryDuration, retryRetryCount int32
		var eventTypes []string
		var filterHeaders, filterBody []byte
		var filterIsFlattened pgtype.Bool
		var filterRawHeaders, filterRawBody []byte
		var rateLimitCount, rateLimitDuration int32

		if err := rows.Scan(
			&name, &id, &subType, &projectID, &endpointID, &deviceID, &sourceID,
			&function, &deliveryMode, &updatedAt, &createdAt,
			&alertCount, &alertThreshold,
			&retryType, &retryDuration, &retryRetryCount,
			&eventTypes, &filterHeaders, &filterBody, &filterIsFlattened,
			&filterRawHeaders, &filterRawBody,
			&rateLimitCount, &rateLimitDuration,
		); err != nil {
			s.logger.WithError(err).Error("failed to scan updated subscription")
			return nil, &ServiceError{ErrMsg: "failed to scan updated subscription", Err: err}
		}

		sub := datastore.Subscription{
			UID:             id,
			Name:            name,
			Type:            datastore.SubscriptionType(subType),
			ProjectID:       projectID,
			EndpointID:      pgTextToString(endpointID),
			DeviceID:        pgTextToString(deviceID),
			SourceID:        pgTextToString(sourceID),
			Function:        null.NewString(pgTextToString(function), function.Valid),
			DeliveryMode:    datastore.DeliveryMode(pgTextToString(deliveryMode)),
			AlertConfig:     paramsToAlertConfig(alertCount, alertThreshold),
			RetryConfig:     paramsToRetryConfig(retryType, retryDuration, retryRetryCount),
			FilterConfig:    paramsToFilterConfig(eventTypes, filterHeaders, filterBody, filterIsFlattened, filterRawHeaders, filterRawBody),
			RateLimitConfig: paramsToRateLimitConfig(rateLimitCount, rateLimitDuration),
			CreatedAt:       pgTimestamptzToTime(createdAt),
			UpdatedAt:       pgTimestamptzToTime(updatedAt),
		}

		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		s.logger.WithError(err).Error("failed to iterate updated subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to iterate updated subscriptions", Err: err}
	}

	return subs, nil
}

func (s *Service) FetchNewSubscriptions(ctx context.Context, projectIDs, knownSubscriptionIDs []string, lastSyncTime time.Time, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	// If no known subscriptions and lastSyncTime is zero, return empty
	if len(knownSubscriptionIDs) == 0 && lastSyncTime.IsZero() {
		return []datastore.Subscription{}, nil
	}

	hasKnownIDs := len(knownSubscriptionIDs) > 0
	if !hasKnownIDs {
		knownSubscriptionIDs = []string{}
	}

	rows, err := s.repo.FetchNewSubscriptions(ctx, repo.FetchNewSubscriptionsParams{
		LastSyncTime:         pgtype.Timestamptz{Time: lastSyncTime, Valid: true},
		HasKnownIds:          hasKnownIDs,
		KnownSubscriptionIds: knownSubscriptionIDs,
		ProjectIds:           projectIDs,
		LimitVal:             int64(pageSize),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch new subscriptions")
		return nil, &ServiceError{ErrMsg: "failed to fetch new subscriptions", Err: err}
	}

	subs := make([]datastore.Subscription, 0, len(rows))
	for _, row := range rows {
		sub := &datastore.Subscription{
			Name:       row.Name,
			UID:        row.ID,
			Type:       datastore.SubscriptionType(row.Type),
			ProjectID:  row.ProjectID,
			EndpointID: pgTextToString(row.EndpointID),
			Function:   null.NewString(pgTextToString(row.Function), row.Function.Valid),
			UpdatedAt:  pgTimestamptzToTime(row.UpdatedAt),
			FilterConfig: paramsToFilterConfig(
				row.FilterConfigEventTypes,
				row.FilterConfigFilterHeaders,
				row.FilterConfigFilterBody,
				row.FilterConfigFilterIsFlattened,
				row.FilterConfigFilterRawHeaders,
				row.FilterConfigFilterRawBody,
			),
		}
		subs = append(subs, *sub)
	}

	return subs, nil
}

// ============================================================================
// UTILITY Operations
// ============================================================================

func (s *Service) CountEndpointSubscriptions(ctx context.Context, projectID, endpointID, subscriptionID string) (int64, error) {
	count, err := s.repo.CountEndpointSubscriptions(ctx, repo.CountEndpointSubscriptionsParams{
		ProjectID:             projectID,
		EndpointID:            stringToPgText(endpointID),
		ExcludeSubscriptionID: subscriptionID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to count endpoint subscriptions")
		return 0, &ServiceError{ErrMsg: "failed to count endpoint subscriptions", Err: err}
	}

	return count, nil
}

func (s *Service) TestSubscriptionFilter(_ context.Context, payload, filter interface{}, isFlattened bool) (bool, error) {
	// If filter is nil, match everything
	if filter == nil {
		return true, nil
	}

	// If payload is nil but filter is not, no match
	if payload == nil {
		return false, nil
	}

	p, err := flatten.Flatten(payload)
	if err != nil {
		return false, err
	}

	if !isFlattened {
		filter, err = flatten.Flatten(filter)
		if err != nil {
			return false, err
		}
	}

	v, ok := filter.(flatten.M)
	if !ok {
		return false, fmt.Errorf("unknown type %T for filter", filter)
	}
	return compare.Compare(p, v)
}

func (s *Service) CompareFlattenedPayload(_ context.Context, payload, filter flatten.M, isFlattened bool) (bool, error) {
	// If filter is nil, match everything
	if filter == nil {
		return true, nil
	}

	// If payload is nil but filter is not, no match
	if payload == nil {
		return false, nil
	}

	if !isFlattened {
		var err error
		filter, err = flatten.Flatten(filter)
		if err != nil {
			return false, err
		}
	}

	return compare.Compare(payload, filter)
}
