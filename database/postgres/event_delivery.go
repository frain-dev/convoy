package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
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
        ed.id,ed.project_id,ed.event_id,ed.subscription_id,
        ed.headers,ed.attempts,ed.status,ed.metadata,ed.cli_metadata,
        ed.description,ed.created_at,ed.updated_at,
        COALESCE(ed.device_id,'') as "device_id",
        COALESCE(ed.endpoint_id,'') as "endpoint_id",
        ep.id as "endpoint_metadata.id",
        ep.title as "endpoint_metadata.title",
        ep.project_id as "endpoint_metadata.project_id",
        ep.support_email as "endpoint_metadata.support_email",
        ep.target_url as "endpoint_metadata.target_url",
        ev.id as "event_metadata.id",
        ev.event_type as "event_metadata.event_type",
        COALESCE(d.host_name,'') as "cli_metadata.host_name",
		COALESCE(s.id, '') AS "source_metadata.id",
		COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.event_deliveries ed 
	LEFT JOIN convoy.endpoints ep ON ed.endpoint_id = ep.id 
	LEFT JOIN convoy.events ev ON ed.event_id = ev.id
    LEFT JOIN convoy.devices d ON ed.device_id = d.id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
	WHERE ed.deleted_at IS NULL
    `

	baseEventDeliveryPagedForward = `
	%s 
	%s 
	AND ed.id <= :cursor 
	GROUP BY ed.id, ep.id, ev.id, d.host_name, s.id
	ORDER BY ed.id DESC 
	LIMIT :limit
	`

	baseEventDeliveryPagedBackward = `
	WITH event_deliveries AS (  
		%s 
		%s 
		AND ed.id >= :cursor 
		GROUP BY ed.id, ep.id, ev.id, d.host_name, s.id
		ORDER BY ed.id ASC 
		LIMIT :limit
	)

	SELECT * FROM event_deliveries ORDER BY id DESC
	`

	fetchEventDeliveryByID = baseFetchEventDelivery + ` AND ed.id = $1 AND ed.project_id = $2`

	baseEventDeliveryFilter = ` AND ed.project_id = :project_id 
	AND (ed.event_id = :event_id OR :event_id = '') 
	AND ed.created_at >= :start_date 
	AND ed.created_at <= :end_date
	AND ed.deleted_at IS NULL`

	countPrevEventDeliveries = `
	SELECT count(distinct(ed.id)) as count
	FROM convoy.event_deliveries ed
	WHERE ed.deleted_at IS NULL
	%s
	AND ed.id > :cursor GROUP BY ed.id ORDER BY ed.id DESC LIMIT 1`

	loadEventDeliveriesIntervals = `
    SELECT
        date_trunc('%s', created_at) as "data.group_only",
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
        "data.group_only", "data.index"
    ORDER BY
        "data.total_time" ASC;
    `

	fetchEventDeliveries = `
    SELECT
        id,project_id,event_id,subscription_id,
        headers,attempts,status,metadata,cli_metadata,
        description,created_at,updated_at,
        COALESCE(device_id,'') as "device_id",
        COALESCE(endpoint_id,'') as "endpoint_id"
    FROM convoy.event_deliveries ed WHERE %s AND deleted_at IS NULL;
    `

	fetchDiscardedEventDeliveries = `
    SELECT
        id,project_id,event_id,subscription_id,
        headers,attempts,status,metadata,cli_metadata,
        description,created_at,updated_at,
        COALESCE(device_id,'') as "device_id",
        COALESCE(endpoint_id,'') as "endpoint_id"
    FROM convoy.event_deliveries 
	WHERE status=$1 AND project_id = $2 AND device_id = $3 
	AND (endpoint_id = $4 or $4 = '') 
	AND created_at >= $5 AND created_at <= $6
	AND deleted_at IS NULL;
    `

	countEventDeliveriesByStatus = `
    SELECT COUNT(id) FROM convoy.event_deliveries WHERE status = $1 AND (project_id = $2 OR $2 = '') AND created_at >= $3 AND created_at <= $4 AND deleted_at IS NULL;
    `

	countEventDeliveries = `
    SELECT COUNT(id) FROM convoy.event_deliveries WHERE (project_id = ? OR ? = '') AND (event_id = ? OR ? = '') AND created_at >= ? AND created_at <= ? AND deleted_at IS NULL
    `

	updateEventDeliveriesStatus = `
    UPDATE convoy.event_deliveries SET status = ?, updated_at = now() WHERE (project_id = ? OR ? = '')AND id IN (?) AND deleted_at IS NULL;
    `

	updateEventDeliveryAttempts = `
    UPDATE convoy.event_deliveries SET attempts = $1, status = $2, metadata = $3,  updated_at = now() WHERE id = $4 AND project_id = $5 AND deleted_at IS NULL;
    `

	softDeleteProjectEventDeliveries = `
    UPDATE convoy.event_deliveries SET deleted_at = now() WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL;
    `

	hardDeleteProjectEventDeliveries = `
    DELETE FROM convoy.event_deliveries WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL;
    `
)

func NewEventDeliveryRepo(db database.Database) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{db: db.GetDB()}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
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
		delivery.SubscriptionID, delivery.Headers, delivery.DeliveryAttempts, delivery.Status,
		delivery.Metadata, delivery.CLIMetadata, delivery.Description, delivery.CreatedAt, delivery.UpdatedAt,
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

func (e *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, projectID string, id string) (*datastore.EventDelivery, error) {
	eventDelivery := &datastore.EventDelivery{}
	err := e.db.QueryRowxContext(ctx, fetchEventDeliveryByID, id, projectID).StructScan(eventDelivery)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEventDeliveryNotFound
		}
		return nil, err
	}

	return eventDelivery, nil
}

func (e *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, projectID string, ids []string) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	query, args, err := sqlx.In(fmt.Sprintf(fetchEventDeliveries, "id IN (?) AND project_id = ?"), ids, projectID)
	if err != nil {
		return nil, err
	}

	query = e.db.Rebind(query)

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ed datastore.EventDelivery
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, projectID string, eventID string) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	q := fmt.Sprintf(fetchEventDeliveries, "event_id = $1 AND project_id = $2")
	rows, err := e.db.QueryxContext(ctx, q, eventID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ed datastore.EventDelivery
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) CountDeliveriesByStatus(ctx context.Context, projectID string, status datastore.EventDeliveryStatus, params datastore.SearchParams) (int64, error) {
	count := struct {
		Count int64
	}{}

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)
	err := e.db.QueryRowxContext(ctx, countEventDeliveriesByStatus, status, projectID, start, end).StructScan(&count)
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

func (e *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, projectID string, delivery datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
	query, args, err := sqlx.In(updateEventDeliveriesStatus, status, projectID, projectID, []string{delivery.UID})
	if err != nil {
		return err
	}

	query = e.db.Rebind(query)

	result, err := e.db.ExecContext(ctx, query, args...)
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

func (e *eventDeliveryRepo) UpdateStatusOfEventDeliveries(ctx context.Context, projectID string, ids []string, status datastore.EventDeliveryStatus) error {
	query, args, err := sqlx.In(updateEventDeliveriesStatus, status, projectID, projectID, ids)
	if err != nil {
		return err
	}

	query = e.db.Rebind(query)

	result, err := e.db.ExecContext(ctx, query, args...)
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

func (e *eventDeliveryRepo) FindDiscardedEventDeliveries(ctx context.Context, projectID, endpointID, deviceId string, searchParams datastore.SearchParams) ([]datastore.EventDelivery, error) {
	eventDeliveries := make([]datastore.EventDelivery, 0)

	start := time.Unix(searchParams.CreatedAtStart, 0)
	end := time.Unix(searchParams.CreatedAtEnd, 0)

	rows, err := e.db.QueryxContext(ctx, fetchDiscardedEventDeliveries, datastore.DiscardedEventStatus, projectID, deviceId, endpointID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ed datastore.EventDelivery
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, err
		}

		eventDeliveries = append(eventDeliveries, ed)
	}

	return eventDeliveries, nil
}

func (e *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, projectID string, delivery datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	result, err := e.db.ExecContext(ctx, updateEventDeliveryAttempts, delivery.DeliveryAttempts, delivery.Status, delivery.Metadata, delivery.UID, projectID)
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

	args := []interface{}{
		projectID, projectID,
		eventID, eventID,
		start, end,
	}

	q := countEventDeliveries

	if len(endpointIDs) > 0 {
		q += ` AND endpoint_id IN (?)`
		args = append(args, endpointIDs)
	}

	if len(status) > 0 {
		q += ` AND status IN (?)`
		args = append(args, status)
	}

	query, args, err := sqlx.In(q, args...)
	if err != nil {
		return 0, err
	}

	query = e.db.Rebind(query)

	err = e.db.QueryRowxContext(ctx, query, args...).StructScan(&count)
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

func (e *eventDeliveryRepo) DeleteProjectEventDeliveries(ctx context.Context, projectID string, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	var result sql.Result
	var err error

	start := time.Unix(filter.CreatedAtStart, 0)
	end := time.Unix(filter.CreatedAtEnd, 0)

	if hardDelete {
		result, err = e.db.ExecContext(ctx, hardDeleteProjectEventDeliveries, projectID, start, end)
	} else {
		result, err = e.db.ExecContext(ctx, softDeleteProjectEventDeliveries, projectID, start, end)
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

	arg := map[string]interface{}{
		"endpoint_ids": endpointIDs,
		"project_id":   projectID,
		"limit":        pageable.Limit(),
		"start_date":   start,
		"event_id":     eventID,
		"end_date":     end,
		"status":       status,
		"cursor":       pageable.Cursor(),
	}

	var query, filterQuery string
	if pageable.Direction == datastore.Next {
		query = baseEventDeliveryPagedForward
	} else {
		query = baseEventDeliveryPagedBackward
	}

	filterQuery = baseEventDeliveryFilter
	if len(endpointIDs) > 0 {
		filterQuery += ` AND ed.endpoint_id IN (:endpoint_ids)`
	}

	if len(status) > 0 {
		filterQuery += ` AND ed.status IN (:status)`
	}

	query = fmt.Sprintf(query, baseFetchEventDelivery, filterQuery)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = e.db.Rebind(query)

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	for rows.Next() {
		var ed EventDeliveryPaginated
		err = rows.StructScan(&ed)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		eventDeliveriesP = append(eventDeliveriesP, ed)
	}

	eventDeliveries := make([]datastore.EventDelivery, 0, len(eventDeliveriesP))

	for i := range eventDeliveriesP {
		ev := &eventDeliveriesP[i]
		var cli *datastore.CLIMetadata
		if ev.CLIMetadata != nil {
			cli = &datastore.CLIMetadata{HostName: ev.CLIMetadata.HostName.ValueOrZero()}
		}
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
			Source: &datastore.Source{
				UID:  ev.Source.UID.ValueOrZero(),
				Name: ev.Source.Name.ValueOrZero(),
			},
			Event:            &datastore.Event{EventType: datastore.EventType(ev.Event.EventType.ValueOrZero())},
			DeliveryAttempts: ev.DeliveryAttempts,
			Status:           ev.Status,
			Metadata:         ev.Metadata,
			CLIMetadata:      cli,
			Description:      ev.Description,
			CreatedAt:        ev.CreatedAt,
			UpdatedAt:        ev.UpdatedAt,
			DeletedAt:        ev.DeletedAt,
		})
	}

	var count datastore.PrevRowCount
	if len(eventDeliveries) > 0 {
		var countQuery string
		var qargs []interface{}
		first := eventDeliveries[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevEventDeliveries, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery, qargs, err = sqlx.In(countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = e.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := e.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
		rows.Close()
	}

	ids := make([]string, len(eventDeliveries))
	for i := range eventDeliveries {
		ids[i] = eventDeliveries[i].UID
	}

	if len(eventDeliveries) > pageable.PerPage {
		eventDeliveries = eventDeliveries[:len(eventDeliveries)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return eventDeliveries, *pagination, nil
}

const (
	dailyIntervalFormat   = "yyyy-mm-dd"        // 1 day
	weeklyIntervalFormat  = dailyIntervalFormat // 1 week
	monthlyIntervalFormat = "yyyy-mm"           // 1 month
	yearlyIntervalFormat  = "yyyy"              // 1 month
)

func (e *eventDeliveryRepo) LoadEventDeliveriesIntervals(ctx context.Context, projectID string, params datastore.SearchParams, period datastore.Period, t int) ([]datastore.EventInterval, error) {
	intervals := make([]datastore.EventInterval, 0)

	start := time.Unix(params.CreatedAtStart, 0)
	end := time.Unix(params.CreatedAtEnd, 0)

	var timeComponent string
	var format string
	var extract string
	switch period {
	case datastore.Daily:
		timeComponent = "day"
		format = dailyIntervalFormat
		extract = "doy"
	case datastore.Weekly:
		timeComponent = "week"
		format = weeklyIntervalFormat
		extract = timeComponent
	case datastore.Monthly:
		timeComponent = "month"
		format = monthlyIntervalFormat
		extract = timeComponent
	case datastore.Yearly:
		timeComponent = "year"
		format = yearlyIntervalFormat
		extract = timeComponent
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}

	q := fmt.Sprintf(loadEventDeliveriesIntervals, timeComponent, timeComponent, format, extract)
	rows, err := e.db.QueryxContext(ctx, q, projectID, start, end)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var interval datastore.EventInterval
		err = rows.StructScan(&interval)
		if err != nil {
			return nil, err
		}

		intervals = append(intervals, interval)
	}

	if len(intervals) < minLen {
		var d time.Duration
		switch period {
		case datastore.Daily:
			d = time.Hour * 24
		case datastore.Weekly:
			d = time.Hour * 24 * 7
		case datastore.Monthly:
			d = time.Hour * 24 * 30
		case datastore.Yearly:
			d = time.Hour * 24 * 365
		}
		intervals, err = padIntervals(intervals, d, period)
		if err != nil {
			return nil, err
		}
	}

	return intervals, nil
}

const minLen = 30

func padIntervals(intervals []datastore.EventInterval, duration time.Duration, period datastore.Period) ([]datastore.EventInterval, error) {
	var err error

	var format string

	switch period {
	case datastore.Daily:
		format = "2006-01-02"
	case datastore.Weekly:
		format = "2006-01-02"
	case datastore.Monthly:
		format = "2006-01"
	case datastore.Yearly:
		format = "2006"
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}

	start := time.Now()
	if len(intervals) > 0 {
		start, err = time.Parse(format, intervals[0].Data.Time)
		if err != nil {
			return nil, err
		}
		start = start.Add(-duration) // take it back once here, since we getting it from the original slice
	}

	numPadding := minLen - (len(intervals))
	paddedIntervals := make([]datastore.EventInterval, numPadding, numPadding+len(intervals))
	for i := numPadding; i > 0; i-- {
		paddedIntervals[i-1] = datastore.EventInterval{
			Data: datastore.EventIntervalData{
				Interval: 0,
				Time:     start.Format(format),
			},
			Count: 0,
		}
		start = start.Add(-duration)
	}

	paddedIntervals = append(paddedIntervals, intervals...)

	return paddedIntervals, nil
}

type EndpointMetadata struct {
	UID          null.String `db:"id"`
	Title        null.String `db:"title"`
	TargetURL    null.String `db:"target_url"`
	ProjectID    null.String `db:"project_id"`
	SupportEmail null.String `db:"support_email"`
}

type EventMetadata struct {
	UID       null.String `db:"id"`
	EventType null.String `db:"event_type"`
}

type SourceMetadata struct {
	UID  null.String `db:"id"`
	Name null.String `db:"name"`
}

type CLIMetadata struct {
	HostName null.String `json:"host_name" db:"host_name"`
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
	Source   *SourceMetadata   `json:"source_metadata,omitempty" db:"source_metadata"`

	DeliveryAttempts datastore.DeliveryAttempts    `json:"-" db:"attempts"`
	Status           datastore.EventDeliveryStatus `json:"status" db:"status"`
	Metadata         *datastore.Metadata           `json:"metadata" db:"metadata"`
	CLIMetadata      *CLIMetadata                  `json:"cli_metadata" db:"cli_metadata"`
	Description      string                        `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time                     `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt        time.Time                     `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt        null.Time                     `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func (m *CLIMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return nil
}
