package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
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
	filter_config_filter_is_flattened,
	rate_limit_config_count,rate_limit_config_duration,function,
	filter_config_filter_raw_headers, filter_config_filter_raw_body,
	delivery_mode
	)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,
        CASE
            WHEN $22 = '' OR $22 IS NULL THEN 'at_least_once'::convoy.delivery_mode
            ELSE $22::convoy.delivery_mode
        END
    );
    `

	updateSubscription = `
        UPDATE convoy.subscriptions SET
        name=$3,
          endpoint_id=$4,
         source_id=$5,
        alert_config_count=$6,
        alert_config_threshold=$7,
        retry_config_type=$8,
        retry_config_duration=$9,
        retry_config_retry_count=$10,
        filter_config_event_types=$11,
        filter_config_filter_headers=$12,
        filter_config_filter_body=$13,
        filter_config_filter_is_flattened=$14,
        rate_limit_config_count=$15,
        rate_limit_config_duration=$16,
        function=$17,
        filter_config_filter_raw_headers=$18,
        filter_config_filter_raw_body=$19,
        delivery_mode=CASE
            WHEN $20 = '' OR $20 IS NULL THEN 'at_least_once'::convoy.delivery_mode
            ELSE $20::convoy.delivery_mode
        END,
        updated_at=now()
        WHERE id = $1 AND project_id = $2
        AND deleted_at IS NULL;
        `

	baseFetchSubscription = `
        SELECT
        s.id,s.name,s.type,
        s.project_id,
        s.created_at,
        s.updated_at, s.function,
        s.delivery_mode AS "delivery_mode",

        COALESCE(s.endpoint_id,'') AS "endpoint_id",
        COALESCE(s.device_id,'') AS "device_id",
        COALESCE(s.source_id,'') AS "source_id",

        s.alert_config_count AS "alert_config.count",
        s.alert_config_threshold AS "alert_config.threshold",
        s.retry_config_type AS "retry_config.type",
        s.retry_config_duration AS "retry_config.duration",
        s.retry_config_retry_count AS "retry_config.retry_count",
        s.filter_config_event_types AS "filter_config.event_types",

        s.filter_config_filter_raw_headers AS "filter_config.filter.raw_headers",
        s.filter_config_filter_raw_body AS "filter_config.filter.raw_body",
        s.filter_config_filter_is_flattened AS "filter_config.filter.is_flattened",
        s.filter_config_filter_headers AS "filter_config.filter.headers",
        s.filter_config_filter_body AS "filter_config.filter.body",

        s.rate_limit_config_count AS "rate_limit_config.count",
        s.rate_limit_config_duration AS "rate_limit_config.duration",

        CASE
        WHEN em.is_encrypted THEN
            COALESCE(pgp_sym_decrypt(em.secrets_cipher::bytea, '%s')::jsonb, '[]'::jsonb)
        ELSE
            COALESCE(em.secrets, '[]'::jsonb)
        END AS "endpoint_metadata.secrets",
        COALESCE(em.id,'') AS "endpoint_metadata.id",
        COALESCE(em.name,'') AS "endpoint_metadata.name",
        COALESCE(em.project_id,'') AS "endpoint_metadata.project_id",
        COALESCE(em.support_email,'') AS "endpoint_metadata.support_email",
        COALESCE(em.url,'') AS "endpoint_metadata.url",
        COALESCE(em.status, '') AS "endpoint_metadata.status",
        COALESCE(em.owner_id, '') AS "endpoint_metadata.owner_id",

        COALESCE(d.id,'') AS "device_metadata.id",
        COALESCE(d.status,'') AS "device_metadata.status",
        COALESCE(d.host_name,'') AS "device_metadata.host_name",

        COALESCE(sm.id,'') AS "source_metadata.id",
        COALESCE(sm.name,'') AS "source_metadata.name",
        COALESCE(sm.type,'') AS "source_metadata.type",
        COALESCE(sm.mask_id,'') AS "source_metadata.mask_id",
        COALESCE(sm.project_id,'') AS "source_metadata.project_id",
         COALESCE(sm.is_disabled,FALSE) AS "source_metadata.is_disabled",

        COALESCE(sv.type, '') AS "source_metadata.verifier.type",
        COALESCE(sv.basic_username, '') AS "source_metadata.verifier.basic_auth.username",
        COALESCE(sv.basic_password, '') AS "source_metadata.verifier.basic_auth.password",
        COALESCE(sv.api_key_header_name, '') AS "source_metadata.verifier.api_key.header_name",
        COALESCE(sv.api_key_header_value, '') AS "source_metadata.verifier.api_key.header_value",
        COALESCE(sv.hmac_hash, '') AS "source_metadata.verifier.hmac.hash",
        COALESCE(sv.hmac_header, '') AS "source_metadata.verifier.hmac.header",
        COALESCE(sv.hmac_secret, '') AS "source_metadata.verifier.hmac.secret",
        COALESCE(sv.hmac_encoding, '') AS "source_metadata.verifier.hmac.encoding"

        FROM convoy.subscriptions s
        LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
        LEFT JOIN convoy.sources sm ON s.source_id = sm.id
        LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
        LEFT JOIN convoy.devices d ON s.device_id = d.id
        WHERE s.deleted_at IS NULL `

	fetchSubscriptionsForBroadcast = `
    select id, type, project_id, endpoint_id, function,
    filter_config_event_types AS "filter_config.event_types",
    filter_config_filter_headers AS "filter_config.filter.headers",
	filter_config_filter_body AS "filter_config.filter.body",
	filter_config_filter_is_flattened AS "filter_config.filter.is_flattened"
    from convoy.subscriptions
    where (ARRAY[$4] <@ filter_config_event_types OR ARRAY['*'] <@ filter_config_event_types)
    AND id > $1
    AND project_id = $2
    AND deleted_at is null
    ORDER BY id LIMIT $3`

	loadAllSubscriptionsConfiguration = `
    select name, id, type, project_id, endpoint_id, function, updated_at,
    filter_config_event_types AS "filter_config.event_types",
    filter_config_filter_headers AS "filter_config.filter.headers",
	filter_config_filter_body AS "filter_config.filter.body",
	filter_config_filter_is_flattened AS "filter_config.filter.is_flattened"
    from convoy.subscriptions
    where id > ?
    AND project_id IN (?)
    AND deleted_at is null
    ORDER BY id LIMIT ?`

	fetchUpdatedSubscriptions = `
	WITH input_map(id, last_updated_at) AS (
		VALUES %s
	),
	updated_existing AS (
		SELECT s.name, s.id, s.type, s.project_id, s.endpoint_id, s.function, s.updated_at,
		s.filter_config_event_types AS "filter_config.event_types",
		s.filter_config_filter_headers AS "filter_config.filter.headers",
		s.filter_config_filter_body AS "filter_config.filter.body",
		s.filter_config_filter_is_flattened AS "filter_config.filter.is_flattened",
		s.filter_config_filter_raw_headers AS "filter_config.filter.raw_headers",
		s.filter_config_filter_raw_body AS "filter_config.filter.raw_body"
		FROM convoy.subscriptions s
		JOIN input_map m ON s.id = m.id
		WHERE s.updated_at > m.last_updated_at
		AND s.project_id IN (:project_ids)
		AND s.deleted_at IS NULL
	),
	new_subscriptions AS (
		SELECT s.name, s.id, s.type, s.project_id, s.endpoint_id, s.function, s.updated_at,
		s.filter_config_event_types AS "filter_config.event_types",
		s.filter_config_filter_headers AS "filter_config.filter.headers",
		s.filter_config_filter_body AS "filter_config.filter.body",
		s.filter_config_filter_is_flattened AS "filter_config.filter.is_flattened",
		s.filter_config_filter_raw_headers AS "filter_config.filter.raw_headers",
		s.filter_config_filter_raw_body AS "filter_config.filter.raw_body"
		FROM convoy.subscriptions s
		WHERE s.id NOT IN (SELECT id FROM input_map)
		AND s.project_id IN (:project_ids)
		AND s.deleted_at IS NULL
	)
	SELECT * FROM updated_existing
	UNION ALL
	SELECT * FROM new_subscriptions
	ORDER BY id LIMIT :limit
	`

	fetchDeletedSubscriptions = `
    select id, deleted_at, project_id,
    filter_config_event_types AS "filter_config.event_types"
    from convoy.subscriptions
    where (deleted_at IS NOT NULL)
    AND id IN (?)
    AND project_id IN (?)
    ORDER BY id LIMIT ?`

	baseFetchSubscriptionsPagedForward = `
	%s
	%s
	AND s.id <= :cursor
	GROUP BY s.id, em.id, sm.id, sv.id, d.id
	ORDER BY s.id DESC
	LIMIT :limit
	`

	baseFetchSubscriptionsPagedBackward = `
	WITH subscriptions AS (
		%s
		%s
		AND s.id >= :cursor
		GROUP BY s.id, em.id, sm.id, sv.id, d.id
		ORDER BY s.id ASC
		LIMIT :limit
	)

	SELECT * FROM subscriptions ORDER BY id DESC
	`

	countProjectSubscriptions = `
	SELECT COUNT(s.id) AS count
	FROM convoy.subscriptions s
	WHERE s.deleted_at IS NULL
	AND s.project_id IN (?)`

	countEndpointSubscriptions = `
	SELECT COUNT(s.id) AS count
	FROM convoy.subscriptions s
	WHERE s.deleted_at IS NULL
	AND s.project_id = $1 AND s.endpoint_id = $2 AND s.id <> $3`

	countPrevSubscriptions = `
	SELECT COUNT(DISTINCT(s.id)) AS count
	FROM convoy.subscriptions s
	WHERE s.deleted_at IS NULL
	%s
	AND s.id > :cursor GROUP BY s.id ORDER BY s.id DESC LIMIT 1`

	fetchSubscriptionByID = baseFetchSubscription + ` AND %s = $1 AND %s = $2;`

	fetchSubscriptionByDeviceID = `
    SELECT
    s.id,s.name,s.type,
	s.project_id,
	s.created_at,
	s.updated_at, s.function,

	COALESCE(s.endpoint_id,'') AS "endpoint_id",
	COALESCE(s.device_id,'') AS "device_id",
	COALESCE(s.source_id,'') AS "source_id",

	s.alert_config_count AS "alert_config.count",
	s.alert_config_threshold AS "alert_config.threshold",
	s.retry_config_type AS "retry_config.type",
	s.retry_config_duration AS "retry_config.duration",
	s.retry_config_retry_count AS "retry_config.retry_count",
	s.filter_config_event_types AS "filter_config.event_types",
	s.filter_config_filter_headers AS "filter_config.filter.headers",
	s.filter_config_filter_body AS "filter_config.filter.body",
	s.rate_limit_config_count AS "rate_limit_config.count",
	s.rate_limit_config_duration AS "rate_limit_config.duration",

	COALESCE(d.id,'') AS "device_metadata.id",
	COALESCE(d.status,'') AS "device_metadata.status",
	COALESCE(d.host_name,'') AS "device_metadata.host_name"

	FROM convoy.subscriptions s
	LEFT JOIN convoy.devices d ON s.device_id = d.id
    WHERE s.device_id = $1 AND s.project_id = $2 AND s.type = $3`

	fetchCLISubscriptions = baseFetchSubscription + `AND %s = $1 AND %s = $2`

	deleteSubscriptions = `
	UPDATE convoy.subscriptions SET
	deleted_at = NOW()
	WHERE id = $1 AND project_id = $2;
	`

	upsertSubscriptionEventTypes = `
	INSERT INTO convoy.event_types (id, name, project_id, description, category)
	VALUES (:id, :name, :project_id, :description, :category)
	on conflict do nothing;`

	insertSubscriptionEventTypeFilters = `
	INSERT INTO convoy.filters (
		id,
		subscription_id,
		event_type,
		headers,
		body,
		raw_headers,
		raw_body
	)
	SELECT
		convoy.generate_ulid()::VARCHAR,
		id,
		unnest(filter_config_event_types),
		filter_config_filter_headers,
		filter_config_filter_body,
		filter_config_filter_raw_headers,
		filter_config_filter_raw_body
	FROM convoy.subscriptions
	WHERE deleted_at IS NULL ON CONFLICT DO NOTHING;
	`

	deleteSubscriptionEventTypes = `
	delete from convoy.filters where id in (
		select f.id as filter_id
		from convoy.filters f
		join convoy.subscriptions s on
		s.id = f.subscription_id where s.id = $1
		and f.event_type <> all(s.filter_config_event_types)
	);
	`
)

var (
	ErrSubscriptionNotCreated = errors.New("subscription could not be created")
	ErrSubscriptionNotUpdated = errors.New("subscription could not be updated")
	ErrSubscriptionNotDeleted = errors.New("subscription could not be deleted")
)

type subscriptionRepo struct {
	db database.Database
	km keys.KeyManager
}

func NewSubscriptionRepo(db database.Database) datastore.SubscriptionRepository {
	km, err := keys.Get()
	if err != nil {
		log.Fatal(err)
	}
	return &subscriptionRepo{db: db, km: km}
}

func (s *subscriptionRepo) LoadAllSubscriptionConfig(ctx context.Context, projectIDs []string, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	query, args, err := sqlx.In(countProjectSubscriptions, projectIDs)
	if err != nil {
		return nil, err
	}

	var subCount int64
	err = s.db.GetReadDB().GetContext(ctx, &subCount, s.db.GetReadDB().Rebind(query), args...)
	if err != nil {
		return nil, err
	}

	if subCount == 0 {
		return []datastore.Subscription{}, nil
	}

	subs := make([]datastore.Subscription, subCount)
	cursor := "0"
	var rows *sqlx.Rows // reuse the mem
	counter := 0
	numBatches := int64(math.Ceil(float64(subCount) / float64(pageSize)))

	for i := int64(0); i < numBatches; i++ {
		query, args, err = sqlx.In(loadAllSubscriptionsConfiguration, cursor, projectIDs, pageSize)
		if err != nil {
			return nil, err
		}

		rows, err = s.db.GetReadDB().QueryxContext(ctx, s.db.GetReadDB().Rebind(query), args...)
		if err != nil {
			return nil, err
		}

		// using func to avoid calling defer in a loop, that can easily fill up function stack and cause a crash
		func() {
			defer closeWithError(rows)
			for rows.Next() {
				sub := datastore.Subscription{}
				if err = rows.StructScan(&sub); err != nil {
					return
				}

				nullifyEmptyConfig(&sub)
				subs[counter] = sub
				counter++
			}

			if counter > 0 {
				cursor = subs[counter-1].UID
			}
		}()

		if err != nil {
			return nil, err
		}

	}

	return subs[:counter], nil
}

func (s *subscriptionRepo) FetchSubscriptionsForBroadcast(ctx context.Context, projectID string, eventType string, pageSize int) ([]datastore.Subscription, error) {
	var _subs []datastore.Subscription
	cursor := "0"

	for {
		rows, err := s.db.GetReadDB().QueryxContext(ctx, fetchSubscriptionsForBroadcast, cursor, projectID, pageSize, eventType)
		if err != nil {
			return nil, err
		}

		subscriptions, err := scanSubscriptions(rows)
		if err != nil {
			return nil, err
		}

		if len(subscriptions) == 0 {
			break
		}

		_subs = append(_subs, subscriptions...)
		cursor = subscriptions[len(subscriptions)-1].UID
	}

	return _subs, nil
}

func (s *subscriptionRepo) FetchDeletedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 || len(subscriptionUpdates) == 0 {
		return []datastore.Subscription{}, nil
	}

	ids := make([]string, 0)
	for _, sub := range subscriptionUpdates {
		ids = append(ids, sub.UID)
	}

	query, args, err := sqlx.In(fetchDeletedSubscriptions, ids, projectIDs, pageSize)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.GetReadDB().QueryxContext(ctx, s.db.GetReadDB().Rebind(query), args...)
	if err != nil {
		return nil, err
	}

	defer closeWithError(rows)
	subs := make([]datastore.Subscription, 0)
	for rows.Next() {
		sub := datastore.Subscription{}
		if err = rows.StructScan(&sub); err != nil {
			return nil, err
		}

		nullifyEmptyConfig(&sub)
		subs = append(subs, sub)
	}

	return subs, nil
}

func (s *subscriptionRepo) FetchUpdatedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	if len(projectIDs) == 0 || len(subscriptionUpdates) == 0 {
		return []datastore.Subscription{}, nil
	}

	valuesSQL := ""
	for i := range subscriptionUpdates {
		valuesSQL += fmt.Sprintf("(:id%d, CAST(:last_updated_at%d AS timestamptz))", i, i)
		if i < len(subscriptionUpdates)-1 {
			valuesSQL += ", "
		}
	}

	query := fmt.Sprintf(fetchUpdatedSubscriptions, valuesSQL)

	//Flatten named params into a single map
	args := map[string]interface{}{}
	for i, e := range subscriptionUpdates {
		args[fmt.Sprintf("id%d", i)] = e.UID
		args[fmt.Sprintf("last_updated_at%d", i)] = e.UpdatedAt
	}

	args["project_ids"] = projectIDs
	args["limit"] = pageSize

	// Use sqlx.Named to bind the named parameters
	query, finalArgs, err := sqlx.Named(query, args)
	if err != nil {
		return nil, err
	}

	query, finalArgs, err = sqlx.In(query, finalArgs...)
	if err != nil {
		return nil, err
	}

	query = s.db.GetReadDB().Rebind(query)

	rows, err := s.db.GetReadDB().QueryxContext(ctx, query, finalArgs...)
	if err != nil {
		return nil, err
	}

	subs := make([]datastore.Subscription, 0)
	// using func to avoid calling defer in a loop, that can easily fill up function stack and cause a crash
	defer closeWithError(rows)
	for rows.Next() {
		sub := datastore.Subscription{}
		if err = rows.StructScan(&sub); err != nil {
			return nil, err
		}

		nullifyEmptyConfig(&sub)
		subs = append(subs, sub)
	}

	if err != nil {
		return nil, err
	}

	return subs, nil
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
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

	err = fc.Filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = fc.Filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	fc.Filter.IsFlattened = true // this is just a flag so we can identify old records

	tx, err := s.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		innerErr := tx.Rollback()
		if innerErr != nil {
			err = innerErr
		}
	}(tx)

	result, err := tx.ExecContext(
		ctx, createSubscription, subscription.UID,
		subscription.Name, subscription.Type, subscription.ProjectID,
		endpointID, deviceID, sourceID,
		ac.Count, ac.Threshold, rc.Type, rc.Duration, rc.RetryCount,
		fc.EventTypes, fc.Filter.Headers, fc.Filter.Body, fc.Filter.IsFlattened,
		rlc.Count, rlc.Duration, subscription.Function,
		subscription.FilterConfig.Filter.RawHeaders, subscription.FilterConfig.Filter.RawBody,
		subscription.DeliveryMode,
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

	_subscription := &datastore.Subscription{}
	err = tx.QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, key, "s.id", "s.project_id"), subscription.UID, projectID).StructScan(_subscription)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return datastore.ErrSubscriptionNotFound
		}
		return err
	}

	subscription.FilterConfig.Filter.Headers = subscription.FilterConfig.Filter.RawHeaders
	subscription.FilterConfig.Filter.Body = subscription.FilterConfig.Filter.RawBody

	eventTypesSlice := make([]*datastore.ProjectEventType, len(subscription.FilterConfig.EventTypes))
	for i := range subscription.FilterConfig.EventTypes {
		eventTypesSlice[i] = &datastore.ProjectEventType{
			UID:       ulid.Make().String(),
			Name:      subscription.FilterConfig.EventTypes[i],
			ProjectId: subscription.ProjectID,
		}
	}

	// create event types for each subscription
	_, err = tx.NamedExecContext(ctx, upsertSubscriptionEventTypes, eventTypesSlice)
	if err != nil {
		return err
	}

	// create filters for each event type
	_, err = tx.ExecContext(ctx, insertSubscriptionEventTypeFilters)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	ac := subscription.GetAlertConfig()
	rc := subscription.GetRetryConfig()
	fc := subscription.GetFilterConfig()
	rlc := subscription.GetRateLimitConfig()

	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	var sourceID *string
	if !util.IsStringEmpty(subscription.SourceID) {
		sourceID = &subscription.SourceID
	}

	err = fc.Filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = fc.Filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	fc.Filter.IsFlattened = true // this is just a flag so we can identify old records

	tx, err := s.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		innerErr := tx.Rollback()
		if innerErr != nil {
			err = innerErr
		}
	}(tx)

	result, err := s.db.GetDB().ExecContext(
		ctx, updateSubscription, subscription.UID, projectID,
		subscription.Name, subscription.EndpointID, sourceID,
		ac.Count, ac.Threshold, rc.Type, rc.Duration, rc.RetryCount,
		fc.EventTypes, fc.Filter.Headers, fc.Filter.Body, fc.Filter.IsFlattened,
		rlc.Count, rlc.Duration, subscription.Function,
		fc.Filter.RawHeaders, fc.Filter.RawBody,
		subscription.DeliveryMode,
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

	_subscription := &datastore.Subscription{}
	err = tx.QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, key, "s.id", "s.project_id"), subscription.UID, projectID).StructScan(_subscription)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return datastore.ErrSubscriptionNotFound
		}
		return err
	}

	subscription.FilterConfig.Filter.Headers = subscription.FilterConfig.Filter.RawHeaders
	subscription.FilterConfig.Filter.Body = subscription.FilterConfig.Filter.RawBody

	eventTypesSlice := make([]*datastore.ProjectEventType, len(subscription.FilterConfig.EventTypes))
	for i := range subscription.FilterConfig.EventTypes {
		eventTypesSlice[i] = &datastore.ProjectEventType{
			UID:         ulid.Make().String(),
			Name:        subscription.FilterConfig.EventTypes[i],
			ProjectId:   subscription.ProjectID,
			Description: "",
			Category:    "",
		}
	}

	// create event types for each subscription
	_, err = tx.NamedExecContext(ctx, upsertSubscriptionEventTypes, eventTypesSlice)
	if err != nil {
		return err
	}

	// delete filters when they are removed from the subscription
	_, err = tx.ExecContext(ctx, deleteSubscriptionEventTypes, subscription.UID)
	if err != nil {
		return err
	}

	// create filters for each event type
	_, err = tx.ExecContext(ctx, insertSubscriptionEventTypeFilters)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	var rows *sqlx.Rows
	var err error

	arg := map[string]interface{}{
		"project_id":   projectID,
		"endpoint_ids": filter.EndpointIDs,
		"limit":        pageable.Limit(),
		"cursor":       pageable.Cursor(),
		"name":         fmt.Sprintf("%%%s%%", filter.SubscriptionName),
	}

	var query, filterQuery string
	if pageable.Direction == datastore.Next {
		query = baseFetchSubscriptionsPagedForward
	} else {
		query = baseFetchSubscriptionsPagedBackward
	}

	filterQuery = ` AND s.project_id = :project_id`
	if len(filter.EndpointIDs) > 0 {
		filterQuery += ` AND s.endpoint_id IN (:endpoint_ids)`
	}

	if !util.IsStringEmpty(filter.SubscriptionName) {
		filterQuery += ` AND s.name LIKE :name`
	}

	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = fmt.Sprintf(query, fmt.Sprintf(baseFetchSubscription, key), filterQuery)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = s.db.GetReadDB().Rebind(query)

	query = strings.ReplaceAll(query, ":", "::")

	rows, err = s.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	subscriptions, err := scanSubscriptions(rows)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var prevRowCount datastore.PrevRowCount
	if len(subscriptions) > 0 {
		var countQuery string
		var qargs []interface{}
		first := subscriptions[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevSubscriptions, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery, qargs, err = sqlx.In(countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = s.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err = s.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&prevRowCount)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
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

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	result, err := s.db.GetDB().ExecContext(ctx, deleteSubscriptions, subscription.UID, projectID)
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
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	err = s.db.GetDB().QueryRowxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, key, "s.id", "s.project_id"), subscriptionID, projectID).StructScan(subscription)
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
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.GetDB().QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, key, "s.project_id", "s.source_id"), projectID, sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}

		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) FindSubscriptionsByEndpointID(ctx context.Context, projectId string, endpointID string) ([]datastore.Subscription, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.GetDB().QueryxContext(ctx, fmt.Sprintf(fetchSubscriptionByID, key, "s.project_id", "s.endpoint_id"), projectId, endpointID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}

		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) FindSubscriptionByDeviceID(ctx context.Context, projectId string, deviceID string, subscriptionType datastore.SubscriptionType) (*datastore.Subscription, error) {
	subscription := &datastore.Subscription{}
	err := s.db.GetDB().QueryRowxContext(ctx, fetchSubscriptionByDeviceID, deviceID, projectId, subscriptionType).StructScan(subscription)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSubscriptionNotFound
		}

		return nil, err
	}

	nullifyEmptyConfig(subscription)

	return subscription, nil
}

func (s *subscriptionRepo) FindCLISubscriptions(ctx context.Context, projectID string) ([]datastore.Subscription, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.GetReadDB().QueryxContext(ctx, fmt.Sprintf(fetchCLISubscriptions, key, "s.project_id", "s.type"), projectID, datastore.SubscriptionTypeCLI)
	if err != nil {
		return nil, err
	}

	return scanSubscriptions(rows)
}

func (s *subscriptionRepo) CountEndpointSubscriptions(ctx context.Context, projectID, endpointID, subscriptionID string) (int64, error) {
	var count int64

	err := s.db.GetReadDB().GetContext(ctx, &count, countEndpointSubscriptions, projectID, endpointID, subscriptionID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *subscriptionRepo) TestSubscriptionFilter(_ context.Context, payload, filter interface{}, isFlattened bool) (bool, error) {
	if payload == nil || filter == nil {
		return true, nil
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

	// The filter must be of type flatten.M, because flatten.Flatten always returns that type,
	// so whether pre-flattened or not, this must hold true
	v, ok := filter.(flatten.M)
	if !ok {
		return false, fmt.Errorf("unknown type %T for filter", filter)
	}
	return compare.Compare(p, v)
}

func (s *subscriptionRepo) CompareFlattenedPayload(_ context.Context, payload, filter flatten.M, isFlattened bool) (bool, error) {
	if payload == nil || filter == nil {
		return true, nil
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

var (
	emptyAlertConfig     = datastore.AlertConfiguration{}
	emptyRetryConfig     = datastore.RetryConfiguration{}
	emptyRateLimitConfig = datastore.RateLimitConfiguration{}
)

func nullifyEmptyConfig(sub *datastore.Subscription) {
	if sub.AlertConfig != nil && *sub.AlertConfig == emptyAlertConfig {
		sub.AlertConfig = nil
	}

	if sub.RetryConfig != nil && *sub.RetryConfig == emptyRetryConfig {
		sub.RetryConfig = nil
	}

	if sub.RateLimitConfig != nil && *sub.RateLimitConfig == emptyRateLimitConfig {
		sub.RateLimitConfig = nil
	}
}

func scanSubscriptions(rows *sqlx.Rows) ([]datastore.Subscription, error) {
	subscriptions := make([]datastore.Subscription, 0)
	var err error
	defer closeWithError(rows)

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
