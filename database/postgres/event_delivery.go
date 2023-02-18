package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"
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
    INSERT INTO convoy.event_deliveries (id,project_id,event_id,endpoint_id,device_id,subscription_id,headers,attempts,status,metadata,cli_metadata,description,created_at,updated_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);
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
        d.host_name as "cli_metadata.host_name",
    FROM convoy.event_deliveries ed LEFT JOIN convoy.endpoints ep
    ON ed.endpoint_id = ep.id LEFT JOIN convoy.events ev ON ed.event_id = ev.id
    LEFT JOIN convoy.devices d ON ed.device_id = d.id
    `

	fetchEventDeliveryByID = baseFetchEventDelivery + ` WHERE ed.id = $1 AND ed.deleted_at IS NULL`

	loadEventDeliveriesPaged = baseFetchEventDelivery + ` WHERE (ed.project_id = $1 OR $1 = '') AND (ed.event_id = $2 OR $2 = '') AND (ed.status IN ($3) OR cardinality($3) = 0) AND (ed.endpoint_id IN ($4) OR cardinality($4) = 0) AND ed.created_at >= $5 AND ed.created_at <= $6  AND ed.deleted_at IS NULL LIMIT $7 OFFSET $8`

	loadEventDeliveriesIntervals = `
    SELECT
        to_char(date_trunc('%s', created_at), '%s') as "data.total_time",
        extract(%s from created_at) as "data.index",
        count(*) as count
        FROM
            convoy.event_deliveries
        WHERE
        project_id = $1 AND
        deleted_at IS NULL AND
        created_at >= $2 AND
        created_at <= $3
    GROUP BY
        total_time, index
    ORDER BY
        total_time ASC;
    `

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

	var endpointID *string
	var deviceID *string

	if !util.IsStringEmpty(delivery.EndpointID) {
		endpointID = &delivery.EndpointID
	}

	if !util.IsStringEmpty(delivery.DeviceID) {
		deviceID = &delivery.DeviceID
	}

	result, err := e.db.ExecContext(
		ctx, createEventDelivery, delivery.UID, delivery.ProjectID,
		delivery.EventID, endpointID, deviceID,
		delivery.SubscriptionID, headers, attempts, delivery.Status,
		metadata, cliMetadata, delivery.Description, delivery.CreatedAt, delivery.UpdatedAt,
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
	eventDeliveriesP := make([]EventDeliveryPaginated, 0)

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)
	rows, err := e.db.QueryxContext(ctx, loadEventDeliveriesPaged, projectID, eventID, status, endpointIDs, start, end, pageable.PerPage, getSkip(pageable.Page, pageable.PerPage))
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var ed EventDeliveryPaginated
	for rows.Next() {
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		eventDeliveriesP = append(eventDeliveriesP, ed)
	}

	eventDeliveries := make([]datastore.EventDelivery, 0, len(eventDeliveriesP))

	for i := range eventDeliveriesP {
		ev := &eventDeliveriesP[i]
		eventDeliveries = append(eventDeliveries, datastore.EventDelivery{
			UID:            ev.UID,
			ProjectID:      ev.ProjectID,
			EventID:        ev.EventID,
			EndpointID:     ev.EndpointID,
			DeviceID:       ev.DeviceID,
			SubscriptionID: ev.SubscriptionID,
			Headers:        ev.Headers,
			Endpoint: &datastore.Endpoint{
				UID:          ev.Endpoint.UID.ValueOrZero(),
				ProjectID:    ev.Endpoint.ProjectID.ValueOrZero(),
				TargetURL:    ev.Endpoint.TargetURL.ValueOrZero(),
				Title:        ev.Endpoint.Title.ValueOrZero(),
				SupportEmail: ev.Endpoint.SupportEmail.ValueOrZero(),
			},
			Event:            &datastore.Event{EventType: datastore.EventType(ev.Event.EventType.ValueOrZero())},
			DeliveryAttempts: ev.DeliveryAttempts,
			Status:           ev.Status,
			Metadata:         ev.Metadata,
			CLIMetadata: &datastore.CLIMetadata{
				HostName: ev.CLIMetadata.HostName.ValueOrZero(),
			},
			Description: ev.Description,
			CreatedAt:   ev.CreatedAt,
			UpdatedAt:   ev.UpdatedAt,
			DeletedAt:   ev.DeletedAt,
		})
	}

	return eventDeliveries, datastore.PaginationData{}, nil
}

const (
	dailyIntervalFormat   = "yyyy-mm-dd"          // 1 day
	weeklyIntervalFormat  = monthlyIntervalFormat // 1 week
	monthlyIntervalFormat = "yyyy-mm"             // 1 month
	yearlyIntervalFormat  = "yyyy"                // 1 month
)

func (e *eventDeliveryRepo) LoadEventDeliveriesIntervals(ctx context.Context, projectID string, params datastore.SearchParams, period datastore.Period, i int) ([]datastore.EventInterval, error) {
	intervals := make([]datastore.EventInterval, 0)

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)

	var timeComponent string
	var format string
	switch period {
	case datastore.Daily:
		timeComponent = "day"
		format = dailyIntervalFormat
	case datastore.Weekly:
		timeComponent = "week"
		format = weeklyIntervalFormat
	case datastore.Monthly:
		timeComponent = "month"
		format = monthlyIntervalFormat
	case datastore.Yearly:
		timeComponent = "year"
		format = yearlyIntervalFormat
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}

	q := fmt.Sprintf(loadEventDeliveriesIntervals, timeComponent, format, timeComponent)
	rows, err := e.db.QueryxContext(ctx, q, projectID, start, end)
	if err != nil {
		return nil, err
	}

	var interval datastore.EventInterval
	for rows.Next() {
		err = rows.StructScan(&interval)
		if err != nil {
			return nil, err
		}

		intervals = append(intervals, interval)
	}

	return intervals, nil
}

type EndpointMetadata struct {
	UID          null.String `db:"id"`
	Title        null.String `db:"title"`
	TargetURL    null.String `db:"target_url"`
	ProjectID    null.String `db:"project_id"`
	SupportEmail null.String `db:"support_email"`
}

type EventMetadata struct {
	UID       null.String ` db:"id"`
	EventType null.String `db:"event_type"`
}

type CLIMetadata struct {
	HostName null.String `db:"host_name"`
}

type EventDeliveryPaginated struct {
	UID            string                `json:"uid" db:"id"`
	ProjectID      string                `json:"project_id,omitempty" db:"project_id"`
	EventID        string                `json:"event_id,omitempty" db:"event_id"`
	EndpointID     string                `json:"endpoint_id,omitempty" db:"endpoint_id"`
	DeviceID       string                `json:"device_id" db:"device_id"`
	SubscriptionID string                `json:"subscription_id,omitempty" db:"subscription_id"`
	Headers        httpheader.HTTPHeader `json:"headers" db:"headers"`

	Endpoint *EndpointMetadata `json:"endpoint_metadata,omitempty" db:"endpoint_metadata"`
	Event    *EventMetadata    `json:"event_metadata,omitempty" db:"event_metadata"`

	DeliveryAttempts []datastore.DeliveryAttempt   `json:"-" db:"attempts"`
	Status           datastore.EventDeliveryStatus `json:"status" db:"status"`
	Metadata         *datastore.Metadata           `json:"metadata" db:"metadata"`
	CLIMetadata      *CLIMetadata                  `json:"cli_metadata" db:"cli_metadata"`
	Description      string                        `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time                     `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt        time.Time                     `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt        null.Time                     `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}
