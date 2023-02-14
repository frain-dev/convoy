package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

type eventDeliveryRepo struct {
	db *sqlx.DB
}

var (
	ErrEventDeliveryNotCreated         = errors.New("event delivery could not be created")
	ErrEventDeliveryStatusNotUpdated   = errors.New("event delivery status could not be updated")
	ErrEventDeliveryAttemptsNotUpdated = errors.New("event delivery attempts could not be updated")
	ErrEventDeliveriesNotDeleted       = errors.New("event deliveries could not be deleted")
)

const (
	createEventDelivery = `
    INSERT INTO convoy.event_deliveries (id,project_id,event_id,endpoint_id,device_id,subscription_id,headers,attempts,status,metadata,cli_metadata,description)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);
    `

	baseFetchEventDelivery = `
    SELECT
        ed.*,
        ep.id as "endpoint_metadata.id",
        ep.title as "endpoint_metadata.title",
        ep.project_id as "endpoint_metadata.project_id",
        ep.support_email as "endpoint_metadata.support_email",
        ep.target_url as "endpoint_metadata.target_url",
        ev.id as "event_metadata.id",
        ev.event_type as "event_metadata.event_type",
        d.id as "device_metadata.id",
        d.host_name as "device_metadata.host_name",
    FROM convoy.event_deliveries ed LEFT JOIN convoy.endpoints ep
    ON ed.endpoint_id = ep.id LEFT JOIN convoy.events ev ON ed.event_id = ev.id
    LEFT JOIN convoy.devices d ON ed.device_id = d.id
    `

	fetchEventDeliveryByID = baseFetchEventDelivery + ` WHERE ed.id = $1 AND ed.deleted_at IS NULL`

	loadEventDeliveriesPaged = baseFetchEventDelivery + ` WHERE (ed.project_id = $1 OR $1 = '') AND (ed.event_id = $2 OR $2 = '') AND (ed.status IN ($3) OR cardinality($3) = 0) AND (ed.endpoint_id IN ($4) OR cardinality($4) = 0) AND ed.created_at >= $5 AND ed.created_at <= $6  AND ed.deleted_at IS NULL`

	fetchEventDeliveries = `
    SELECT * FROM convoy.event_deliveries WHERE %s AND deleted_at IS NULL;
    `

	fetchDiscardedEventDeliveries = `
    SELECT * FROM convoy.event_deliveries WHERE status='%s' AND device_id = $1 AND (endpoint_id = $2 or $2 = '') AND created_at >= $5 AND created_at <= $6 AND deleted_at IS NULL;
    `

	countEventDeliveriesByStatus = `
    SELECT COUNT(id) FROM convoy.event_deliveries WHERE status = $1 AND created_at > $2 AND created_at < $3 AND deleted_at IS NULL;
    `

	countEventDeliveries = `
    SELECT COUNT(id) FROM convoy.event_deliveries WHERE (e.project_id = $1 OR $1 = '') AND (e.event_id = $2 OR $2 = '') AND (e.status IN ($3) OR cardinality($3) = 0) AND (e.endpoint_id IN ($4) OR cardinality($4) = 0) AND e.created_at >= $5 AND e.created_at <= $6 AND deleted_at IS NULL;
    `

	updateEventDeliveriesStatus = `
    UPDATE convoy.event_deliveries SET status = $1, updated_at = now() WHERE id IN ($2) AND deleted_at IS NULL;
    `

	updateEventDeliveryAttempts = `
    UPDATE convoy.event_deliveries SET attempts = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL;
    `

	softDeleteProjectEventDeliveries = `
    UPDATE convoy.event_deliveries SET deleted_at = now() WHERE AND project_id = $1 created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL;
    `

	hardDeleteProjectEventDeliveries = `
    DELETE FROM convoy.event_deliveries WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL;
    `
)

func NewEventDeliveryRepo(db *sqlx.DB) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{db: db}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
	delivery.UID = ulid.Make().String()

	headers, err := json.Marshal(delivery.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal event delivery headers: %v", err)
	}

	attempts, err := json.Marshal(delivery.DeliveryAttempts)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery attempts: %v", err)
	}

	metadata, err := json.Marshal(delivery.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal matadata: %v", err)
	}

	cliMetadata, err := json.Marshal(delivery.DeliveryAttempts)
	if err != nil {
		return fmt.Errorf("failed to marshal cli metadata: %v", err)
	}

	result, err := e.db.ExecContext(
		ctx, createEventDelivery, delivery.UID, delivery.ProjectID,
		delivery.EventID, delivery.EndpointID, delivery.DeviceID,
		delivery.SubscriptionID, headers, attempts, delivery.Status,
		metadata, cliMetadata, delivery.Description,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventDeliveryNotCreated
	}

	return nil
}

func (e *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, id string) (*datastore.EventDelivery, error) {
	eventDelivery := &datastore.EventDelivery{}
	err := e.db.QueryRowxContext(ctx, fetchEventDeliveryByID, id).StructScan(eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

func (e *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, ids []string) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	q := fmt.Sprintf(fetchEventDeliveries, "id IN ($1)")
	rows, err := e.db.QueryxContext(ctx, q, ids)
	if err != nil {
		return nil, err
	}

	var ed datastore.EventDelivery
	for rows.Next() {
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, eventID string) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	q := fmt.Sprintf(fetchEventDeliveries, "event_id = $1")
	rows, err := e.db.QueryxContext(ctx, q, eventID)
	if err != nil {
		return nil, err
	}

	var ed datastore.EventDelivery
	for rows.Next() {
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) CountDeliveriesByStatus(ctx context.Context, status datastore.EventDeliveryStatus, params datastore.SearchParams) (int64, error) {
	count := struct {
		Count int64
	}{}

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)
	err := e.db.QueryRowxContext(ctx, countEventDeliveriesByStatus, status, start, end).StructScan(&count)
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

func (e *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, delivery datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
	result, err := e.db.ExecContext(ctx, updateEventDeliveriesStatus, status, []string{delivery.UID})
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventDeliveryStatusNotUpdated
	}

	return nil
}

func (e *eventDeliveryRepo) UpdateStatusOfEventDeliveries(ctx context.Context, ids []string, status datastore.EventDeliveryStatus) error {
	result, err := e.db.ExecContext(ctx, updateEventDeliveriesStatus, status, ids)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventDeliveryStatusNotUpdated
	}

	return nil
}

func (e *eventDeliveryRepo) FindDiscardedEventDeliveries(ctx context.Context, appId, deviceId string, searchParams datastore.SearchParams) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	start := time.Unix(searchParams.CreatedAtStart, 0)
	end := time.Unix(searchParams.CreatedAtEnd, 0)

	q := fmt.Sprintf(fetchDiscardedEventDeliveries, datastore.DiscardedEventStatus)
	rows, err := e.db.QueryxContext(ctx, q, deviceId, appId, start, end)
	if err != nil {
		return nil, err
	}

	var ed datastore.EventDelivery
	for rows.Next() {
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, delivery datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	attempts, err := json.Marshal(delivery.DeliveryAttempts)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery attempts: %v", err)
	}

	result, err := e.db.ExecContext(ctx, updateEventDeliveryAttempts, attempts, delivery.UID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventDeliveryAttemptsNotUpdated
	}

	return nil
}

func (e *eventDeliveryRepo) CountEventDeliveries(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []datastore.EventDeliveryStatus, params datastore.SearchParams) (int64, error) {
	count := struct {
		Count int64
	}{}

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)
	err := e.db.QueryRowxContext(ctx, countEventDeliveries, projectID, eventID, status, endpointIDs, start, end).StructScan(&count)
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

func (e *eventDeliveryRepo) DeleteProjectEventDeliveries(ctx context.Context, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	var result sql.Result
	var err error

	start := time.Unix(filter.CreatedAtStart, 0)
	end := time.Unix(filter.CreatedAtEnd, 0)

	if hardDelete {
		result, err = e.db.ExecContext(ctx, hardDeleteProjectEventDeliveries, filter.ProjectID, start, end)
	} else {
		result, err = e.db.ExecContext(ctx, softDeleteProjectEventDeliveries, filter.ProjectID, start, end)
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventDeliveriesNotDeleted
	}

	return nil
}

func (e *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []datastore.EventDeliveryStatus, params datastore.SearchParams, pageable datastore.Pageable) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)
	rows, err := e.db.QueryxContext(ctx, loadEventDeliveriesPaged, projectID, eventID, status, endpointIDs, start, end)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var ed datastore.EventDelivery
	for rows.Next() {
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, datastore.PaginationData{}, nil
}

func (e *eventDeliveryRepo) LoadEventDeliveriesIntervals(ctx context.Context, s string, params datastore.SearchParams, period datastore.Period, i int) ([]datastore.EventInterval, error) {
	// TODO implement me
	panic("implement me")
}
