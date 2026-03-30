package event_deliveries

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/event_deliveries/repo"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

const PartitionSize = 30_000

type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
	hook   *hooks.Hook
}

var _ datastore.EventDeliveryRepository = (*Service)(nil)

func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
		hook:   db.GetHook(),
	}
}

func (s *Service) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
	if delivery.DeliveryMode == "" {
		delivery.DeliveryMode = datastore.AtLeastOnceDeliveryMode
	}

	params := deliveryToCreateParams(delivery)
	var batchErr error
	br := s.repo.CreateEventDelivery(ctx, []repo.CreateEventDeliveryParams{params})
	br.Exec(func(_ int, err error) {
		if err != nil {
			batchErr = err
		}
	})
	return batchErr
}

func (s *Service) CreateEventDeliveries(ctx context.Context, deliveries []*datastore.EventDelivery) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i := 0; i < len(deliveries); i += PartitionSize {
		end := i + PartitionSize
		if end > len(deliveries) {
			end = len(deliveries)
		}

		chunk := deliveries[i:end]
		params := make([]repo.CreateEventDeliveryParams, len(chunk))
		for j, delivery := range chunk {
			if delivery.DeliveryMode == "" {
				delivery.DeliveryMode = datastore.AtLeastOnceDeliveryMode
			}
			params[j] = deliveryToCreateParams(delivery)
		}

		var batchErr error
		br := repo.New(tx).CreateEventDelivery(ctx, params)
		br.Exec(func(_ int, err error) {
			if err != nil && batchErr == nil {
				batchErr = err
			}
		})
		if batchErr != nil {
			return batchErr
		}
	}

	return tx.Commit(ctx)
}

func (s *Service) FindEventDeliveryByID(ctx context.Context, projectID, id string) (*datastore.EventDelivery, error) {
	params := repo.FindEventDeliveryByIDParams{
		ID:        common.StringToPgTextNullable(id),
		ProjectID: common.StringToPgTextNullable(projectID),
	}
	row, err := s.repo.FindEventDeliveryByID(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEventDeliveryNotFound
		}
		return nil, err
	}
	return rowToEventDelivery(row)
}

func (s *Service) FindEventDeliveryByIDSlim(ctx context.Context, projectID, id string) (*datastore.EventDelivery, error) {
	params := repo.FindEventDeliveryByIDSlimParams{
		ProjectID: common.StringToPgTextNullable(projectID),
		ID:        common.StringToPgTextNullable(id),
	}
	row, err := s.repo.FindEventDeliveryByIDSlim(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEventDeliveryNotFound
		}
		return nil, err
	}
	return rowToEventDelivery(row)
}

func (s *Service) FindEventDeliveriesByIDs(ctx context.Context, projectID string, ids []string) ([]datastore.EventDelivery, error) {
	params := repo.FindEventDeliveriesByIDsParams{
		Ids:       ids,
		ProjectID: common.StringToPgTextNullable(projectID),
	}
	rows, err := s.repo.FindEventDeliveriesByIDs(ctx, params)
	if err != nil {
		return nil, err
	}

	deliveries := make([]datastore.EventDelivery, 0, len(rows))
	for _, row := range rows {
		d, err := rowToEventDelivery(row)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, *d)
	}
	return deliveries, nil
}

func (s *Service) FindEventDeliveriesByEventID(ctx context.Context, projectID, eventID string) ([]datastore.EventDelivery, error) {
	params := repo.FindEventDeliveriesByEventIDParams{
		EventID:   common.StringToPgTextNullable(eventID),
		ProjectID: common.StringToPgTextNullable(projectID),
	}
	rows, err := s.repo.FindEventDeliveriesByEventID(ctx, params)
	if err != nil {
		return nil, err
	}

	deliveries := make([]datastore.EventDelivery, 0, len(rows))
	for _, row := range rows {
		d, err := rowToEventDelivery(row)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, *d)
	}
	return deliveries, nil
}

func (s *Service) CountDeliveriesByStatus(ctx context.Context, projectID string, status datastore.EventDeliveryStatus, params datastore.SearchParams) (int64, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	p := repo.CountDeliveriesByStatusParams{
		Status:    common.StringToPgText(string(status)),
		ProjectID: common.StringToPgText(projectID),
		StartDate: common.TimeToPgTimestamptz(start),
		EndDate:   common.TimeToPgTimestamptz(end),
	}

	count, err := s.repo.CountDeliveriesByStatus(ctx, p)
	if err != nil {
		return 0, err
	}
	return common.PgInt8ToInt64(count), nil
}

func (s *Service) UpdateStatusOfEventDelivery(ctx context.Context, projectID string, delivery datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
	params := repo.UpdateStatusOfEventDeliveriesParams{
		Status:      common.StringToPgText(string(status)),
		Description: common.StringToPgText(delivery.Description),
		ProjectID:   common.StringToPgText(projectID),
		Ids:         []string{delivery.UID},
	}
	return s.repo.UpdateStatusOfEventDeliveries(ctx, params)
}

func (s *Service) UpdateStatusOfEventDeliveries(ctx context.Context, projectID string, ids []string, status datastore.EventDeliveryStatus) error {
	params := repo.UpdateStatusOfEventDeliveriesParams{
		Status:      common.StringToPgText(string(status)),
		Description: common.StringToPgText(""),
		ProjectID:   common.StringToPgText(projectID),
		Ids:         ids,
	}
	return s.repo.UpdateStatusOfEventDeliveries(ctx, params)
}

func (s *Service) FindDiscardedEventDeliveries(ctx context.Context, projectID, deviceID string, searchParams datastore.SearchParams) ([]datastore.EventDelivery, error) {
	start, end := getCreatedDateFilter(searchParams.CreatedAtStart, searchParams.CreatedAtEnd)

	params := repo.FindDiscardedEventDeliveriesParams{
		ProjectID: common.StringToPgTextNullable(projectID),
		DeviceID:  common.StringToPgTextNullable(deviceID),
		StartDate: common.TimeToPgTimestamptz(start),
		EndDate:   common.TimeToPgTimestamptz(end),
	}

	rows, err := s.repo.FindDiscardedEventDeliveries(ctx, params)
	if err != nil {
		return nil, err
	}

	deliveries := make([]datastore.EventDelivery, 0, len(rows))
	for _, row := range rows {
		d, err := rowToEventDelivery(row)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, *d)
	}
	return deliveries, nil
}

func (s *Service) FindStuckEventDeliveriesByStatus(ctx context.Context, status datastore.EventDeliveryStatus) ([]datastore.EventDelivery, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txRepo := repo.New(tx)

	rows, err := txRepo.FindStuckEventDeliveriesByStatus(ctx, common.StringToPgText(string(status)))
	if err != nil {
		return nil, err
	}

	deliveries := make([]datastore.EventDelivery, 0, len(rows))
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		d, err := rowToEventDelivery(row)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, *d)
		ids = append(ids, row.ID)
	}

	// Update status to Processing within the same transaction so other workers
	// won't pick up these rows after the locks are released on commit.
	if len(ids) > 0 {
		err = txRepo.UpdateStatusOfEventDeliveries(ctx, repo.UpdateStatusOfEventDeliveriesParams{
			Status:      common.StringToPgText(string(datastore.ProcessingEventStatus)),
			Description: common.StringToPgText("re-queuing stuck delivery"),
			ProjectID:   common.StringToPgText(""),
			Ids:         ids,
		})
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (s *Service) UpdateEventDeliveryMetadata(ctx context.Context, projectID string, delivery *datastore.EventDelivery) error {
	params := repo.UpdateEventDeliveryMetadataParams{
		Status:         common.StringToPgText(string(delivery.Status)),
		Metadata:       metadataToJSONB(delivery.Metadata),
		LatencySeconds: float64ToNumeric(delivery.LatencySeconds),
		ID:             common.StringToPgTextNullable(delivery.UID),
		ProjectID:      common.StringToPgTextNullable(projectID),
	}

	err := s.repo.UpdateEventDeliveryMetadata(ctx, params)
	if err != nil {
		return err
	}

	s.hook.Fire(ctx, datastore.EventDeliveryUpdated, delivery, nil)
	return nil
}

func (s *Service) CountEventDeliveries(ctx context.Context, projectID string, endpointIDs []string, eventID string,
	status []datastore.EventDeliveryStatus, params datastore.SearchParams) (int64, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	statuses := make([]string, len(status))
	for i, st := range status {
		statuses[i] = string(st)
	}

	p := repo.CountEventDeliveriesParams{
		ProjectID:      common.StringToPgText(projectID),
		EventID:        common.StringToPgText(eventID),
		StartDate:      common.TimeToPgTimestamptz(start),
		EndDate:        common.TimeToPgTimestamptz(end),
		HasEndpointIds: common.BoolToPgBool(len(endpointIDs) > 0),
		EndpointIds:    endpointIDs,
		HasStatus:      common.BoolToPgBool(len(statuses) > 0),
		Statuses:       statuses,
	}

	count, err := s.repo.CountEventDeliveries(ctx, p)
	if err != nil {
		return 0, err
	}
	return common.PgInt8ToInt64(count), nil
}

func (s *Service) DeleteProjectEventDeliveries(ctx context.Context, projectID string, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	start, end := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	if hardDelete {
		return s.repo.HardDeleteProjectEventDeliveries(ctx, repo.HardDeleteProjectEventDeliveriesParams{
			ProjectID: common.StringToPgTextNullable(projectID),
			StartDate: common.TimeToPgTimestamptz(start),
			EndDate:   common.TimeToPgTimestamptz(end),
		})
	}

	return s.repo.SoftDeleteProjectEventDeliveries(ctx, repo.SoftDeleteProjectEventDeliveriesParams{
		ProjectID: common.StringToPgTextNullable(projectID),
		StartDate: common.TimeToPgTimestamptz(start),
		EndDate:   common.TimeToPgTimestamptz(end),
	})
}

func (s *Service) LoadEventDeliveriesPaged(
	ctx context.Context, projectID string, endpointIDs []string, eventID, subscriptionID string,
	status []datastore.EventDeliveryStatus, params datastore.SearchParams, pageable datastore.Pageable,
	idempotencyKey, eventType, brokerMessageId string,
) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	cursor := pageable.Cursor()
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}
	sortOrder := pageable.SortOrder()

	statuses := make([]string, len(status))
	for i, st := range status {
		statuses[i] = string(st)
	}

	p := repo.LoadEventDeliveriesPagedParams{
		SortOrder:          common.StringToPgText(sortOrder),
		ProjectID:          common.StringToPgText(projectID),
		EventID:            common.StringToPgText(eventID),
		EventType:          common.StringToPgText(eventType),
		StartDate:          common.TimeToPgTimestamptz(start),
		EndDate:            common.TimeToPgTimestamptz(end),
		HasEndpointIds:     common.BoolToPgBool(len(endpointIDs) > 0),
		EndpointIds:        endpointIDs,
		HasStatus:          common.BoolToPgBool(len(statuses) > 0),
		Statuses:           statuses,
		HasSubscriptionID:  common.BoolToPgBool(!util.IsStringEmpty(subscriptionID)),
		SubscriptionID:     common.StringToPgText(subscriptionID),
		HasBrokerMessageID: common.BoolToPgBool(!util.IsStringEmpty(brokerMessageId)),
		BrokerMessageID:    common.StringToPgText(brokerMessageId),
		HasIdempotencyKey:  common.BoolToPgBool(!util.IsStringEmpty(idempotencyKey)),
		IdempotencyKey:     common.StringToPgText(idempotencyKey),
		Cursor:             common.StringToPgText(cursor),
		Direction:          common.StringToPgText(direction),
		PageLimit:          pgtype.Int8{Int64: int64(pageable.Limit()), Valid: true},
	}

	rows, err := s.repo.LoadEventDeliveriesPaged(ctx, p)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	deliveries := make([]datastore.EventDelivery, 0, len(rows))
	for _, row := range rows {
		d, err := rowToEventDelivery(row)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		deliveries = append(deliveries, *d)
	}

	// Calculate PrevRowCount if not first page
	var rowCount datastore.PrevRowCount
	isFirstPage := util.IsStringEmpty(cursor)
	if len(deliveries) > 0 && !isFirstPage {
		first := deliveries[0]
		rowCount, err = s.countPrevDeliveries(ctx, projectID, eventID, eventType, endpointIDs,
			statuses, subscriptionID, brokerMessageId, idempotencyKey, first.UID, start, end, sortOrder)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
	}

	ids := make([]string, len(deliveries))
	for i := range deliveries {
		ids[i] = deliveries[i].UID
	}

	pagination := &datastore.PaginationData{PrevRowCount: rowCount}
	pagination = pagination.Build(pageable, ids)

	if len(deliveries) > pageable.PerPage {
		deliveries = deliveries[:len(deliveries)-1]
	}

	return deliveries, *pagination, nil
}

func (s *Service) countPrevDeliveries(ctx context.Context, projectID, eventID, eventType string,
	endpointIDs, statuses []string, subscriptionID, brokerMessageId, idempotencyKey, cursor string,
	start, end time.Time, sortOrder string) (datastore.PrevRowCount, error) {
	params := repo.CountPrevEventDeliveriesParams{
		ProjectID:          common.StringToPgText(projectID),
		EventID:            common.StringToPgText(eventID),
		EventType:          common.StringToPgText(eventType),
		StartDate:          common.TimeToPgTimestamptz(start),
		EndDate:            common.TimeToPgTimestamptz(end),
		HasEndpointIds:     common.BoolToPgBool(len(endpointIDs) > 0),
		EndpointIds:        endpointIDs,
		HasStatus:          common.BoolToPgBool(len(statuses) > 0),
		Statuses:           statuses,
		HasSubscriptionID:  common.BoolToPgBool(!util.IsStringEmpty(subscriptionID)),
		SubscriptionID:     common.StringToPgText(subscriptionID),
		HasBrokerMessageID: common.BoolToPgBool(!util.IsStringEmpty(brokerMessageId)),
		BrokerMessageID:    common.StringToPgText(brokerMessageId),
		HasIdempotencyKey:  common.BoolToPgBool(!util.IsStringEmpty(idempotencyKey)),
		IdempotencyKey:     common.StringToPgText(idempotencyKey),
		SortOrder:          common.StringToPgText(sortOrder),
		Cursor:             common.StringToPgTextNullable(cursor),
	}

	count, err := s.repo.CountPrevEventDeliveries(ctx, params)
	if err != nil {
		return datastore.PrevRowCount{}, err
	}
	return datastore.PrevRowCount{Count: int(count.Int64)}, nil
}

func (s *Service) LoadEventDeliveriesIntervals(ctx context.Context, projectID string, params datastore.SearchParams, period datastore.Period, endpointIds []string) ([]datastore.EventInterval, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	hasEndpoints := common.BoolToPgBool(len(endpointIds) > 0)

	intervalParams := repo.LoadEventDeliveryIntervalsDailyParams{
		ProjectID:      common.StringToPgTextNullable(projectID),
		StartDate:      common.TimeToPgTimestamptz(start),
		EndDate:        common.TimeToPgTimestamptz(end),
		HasEndpointIds: hasEndpoints,
		EndpointIds:    endpointIds,
	}

	// intervalRow is a common shape for all interval query results.
	type intervalRow struct {
		DataIndex     pgtype.Numeric
		DataTotalTime pgtype.Text
		Count         pgtype.Int8
	}

	var rawRows []intervalRow

	switch period {
	case datastore.Daily:
		rows, err := s.repo.LoadEventDeliveryIntervalsDaily(ctx, intervalParams)
		if err != nil {
			return nil, err
		}
		rawRows = make([]intervalRow, len(rows))
		for i, r := range rows {
			rawRows[i] = intervalRow{r.DataIndex, r.DataTotalTime, r.Count}
		}
	case datastore.Weekly:
		rows, err := s.repo.LoadEventDeliveryIntervalsWeekly(ctx, repo.LoadEventDeliveryIntervalsWeeklyParams(intervalParams))
		if err != nil {
			return nil, err
		}
		rawRows = make([]intervalRow, len(rows))
		for i, r := range rows {
			rawRows[i] = intervalRow{r.DataIndex, r.DataTotalTime, r.Count}
		}
	case datastore.Monthly:
		rows, err := s.repo.LoadEventDeliveryIntervalsMonthly(ctx, repo.LoadEventDeliveryIntervalsMonthlyParams(intervalParams))
		if err != nil {
			return nil, err
		}
		rawRows = make([]intervalRow, len(rows))
		for i, r := range rows {
			rawRows[i] = intervalRow{r.DataIndex, r.DataTotalTime, r.Count}
		}
	case datastore.Yearly:
		rows, err := s.repo.LoadEventDeliveryIntervalsYearly(ctx, repo.LoadEventDeliveryIntervalsYearlyParams(intervalParams))
		if err != nil {
			return nil, err
		}
		rawRows = make([]intervalRow, len(rows))
		for i, r := range rows {
			rawRows[i] = intervalRow{r.DataIndex, r.DataTotalTime, r.Count}
		}
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}

	intervals := make([]datastore.EventInterval, 0, len(rawRows))
	for _, r := range rawRows {
		intervals = append(intervals, datastore.EventInterval{
			Data: datastore.EventIntervalData{
				Interval: numericToInt64(r.DataIndex),
				Time:     common.PgTextToString(r.DataTotalTime),
			},
			Count: uint64(common.PgInt8ToInt64(r.Count)),
		})
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
		var err error
		intervals, err = padIntervals(intervals, d, period)
		if err != nil {
			return nil, err
		}
	}

	return intervals, nil
}

// ExportRecords exports event deliveries to a writer as JSONL (one JSON object per line).
// It uses a REPEATABLE READ transaction for snapshot consistency across batches.
func (s *Service) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	// Begin REPEATABLE READ transaction for snapshot consistency
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txRepo := repo.New(tx)

	count, err := txRepo.CountExportedEventDeliveries(ctx, repo.CountExportedEventDeliveriesParams{
		ProjectID: common.StringToPgTextNullable(projectID),
		CreatedAt: common.TimeToPgTimestamptz(createdAt),
		Cursor:    common.StringToPgText(""),
	})
	if err != nil {
		return 0, err
	}

	count64 := common.PgInt8ToInt64(count)
	if count64 == 0 {
		return 0, nil
	}

	var (
		batchSize  = 3000
		numDocs    int64
		numBatches = int(math.Ceil(float64(count64) / float64(batchSize)))
		lastID     string
	)

	for i := 0; i < numBatches; i++ {
		params := repo.ExportEventDeliveriesParams{
			ProjectID: common.StringToPgTextNullable(projectID),
			CreatedAt: common.TimeToPgTimestamptz(createdAt),
			Cursor:    common.StringToPgText(lastID),
			PageLimit: pgtype.Int8{Int64: int64(batchSize), Valid: true},
		}

		rows, exportErr := txRepo.ExportEventDeliveries(ctx, params)
		if exportErr != nil {
			return 0, fmt.Errorf("failed to query batch %d: %w", i, exportErr)
		}

		for _, row := range rows {
			if _, writeErr := w.Write(row.JsonOutput); writeErr != nil {
				return 0, writeErr
			}
			if _, writeErr := w.Write([]byte("\n")); writeErr != nil {
				return 0, writeErr
			}

			numDocs++
			lastID = row.ID
		}
	}

	return numDocs, nil
}

func (s *Service) PartitionEventDeliveriesTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, partitionEventDeliveriesTableSQL)
	return err
}

func (s *Service) UnPartitionEventDeliveriesTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, unPartitionEventDeliveriesTableSQL)
	return err
}

func deliveryToCreateParams(delivery *datastore.EventDelivery) repo.CreateEventDeliveryParams {
	return repo.CreateEventDeliveryParams{
		ID:             common.StringToPgTextNullable(delivery.UID),
		ProjectID:      common.StringToPgTextNullable(delivery.ProjectID),
		EventID:        common.StringToPgTextNullable(delivery.EventID),
		EndpointID:     common.StringPtrToPgTextNullable(emptyToNilStr(delivery.EndpointID)),
		DeviceID:       common.StringPtrToPgTextNullable(emptyToNilStr(delivery.DeviceID)),
		SubscriptionID: common.StringToPgTextNullable(delivery.SubscriptionID),
		Headers:        headersToJSONB(delivery.Headers),
		Status:         common.StringToPgText(string(delivery.Status)),
		Metadata:       metadataToJSONB(delivery.Metadata),
		CliMetadata:    cliMetadataToJSONB(delivery.CLIMetadata),
		Description:    common.StringToPgText(delivery.Description),
		UrlQueryParams: common.StringToPgText(delivery.URLQueryParams),
		IdempotencyKey: common.StringToPgTextNullable(delivery.IdempotencyKey),
		EventType:      common.StringToPgTextNullable(string(delivery.EventType)),
		AcknowledgedAt: common.NullTimeToPgTimestamptz(delivery.AcknowledgedAt),
		DeliveryMode:   string(delivery.DeliveryMode),
	}
}

// emptyToNilStr returns nil for empty strings, pointer otherwise
func emptyToNilStr(s string) *string {
	if util.IsStringEmpty(s) {
		return nil
	}
	return &s
}

// float64ToNumeric converts float64 to pgtype.Numeric
func float64ToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}

// numericToInt64 converts pgtype.Numeric to int64
func numericToInt64(n pgtype.Numeric) int64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return int64(f.Float64)
}

// Partition SQL constants
const partitionEventDeliveriesTableSQL = `
CREATE OR REPLACE FUNCTION enforce_event_delivery_fk()
    RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM convoy.event_deliveries
        WHERE id = NEW.event_delivery_id
    ) THEN
        RAISE EXCEPTION 'Foreign key violation: event_delivery_id % does not exist in event deliveries', NEW.event_delivery_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION partition_event_deliveries_table()
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned event deliveries table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.event_deliveries_new;

    -- Create partitioned table
   create table convoy.event_deliveries_new
    (
        id               VARCHAR not null,
        status           TEXT    not null,
        description      TEXT    not null,
        project_id       VARCHAR not null references convoy.projects,
        endpoint_id      VARCHAR references convoy.endpoints,
        event_id         VARCHAR not null,
        device_id        VARCHAR references convoy.devices,
        subscription_id  VARCHAR not null references convoy.subscriptions,
        metadata         jsonb   not null,
        headers          jsonb,
        attempts         bytea,
        cli_metadata     jsonb,
        created_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        updated_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at       TIMESTAMP WITH TIME ZONE,
        url_query_params VARCHAR,
        idempotency_key  TEXT,
        latency          TEXT,
        event_type       TEXT,
        acknowledged_at  TIMESTAMP WITH TIME ZONE,
        latency_seconds  NUMERIC,
        delivery_mode    convoy.delivery_mode NOT NULL DEFAULT 'at_least_once',
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.event_deliveries
            GROUP BY created_at::DATE, project_id
            order by created_at::DATE
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'event_deliveries_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.event_deliveries_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.event_deliveries_new (
        id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
        attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
        latency_seconds, delivery_mode
    )
    SELECT id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
           attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
           latency_seconds, COALESCE(delivery_mode, 'at_least_once')::convoy.delivery_mode
    FROM convoy.event_deliveries;

    -- Manage table renaming
    ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT IF EXISTS delivery_attempts_event_delivery_id_fkey;
    ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_old;
    ALTER TABLE convoy.event_deliveries_new RENAME TO event_deliveries;
    DROP TABLE IF EXISTS convoy.event_deliveries_old;

    RAISE NOTICE 'Recreating indexes...';
    create index event_deliveries_event_type on convoy.event_deliveries (event_type);
    create index idx_event_deliveries_created_at_key on convoy.event_deliveries (created_at);
    create index idx_event_deliveries_deleted_at_key on convoy.event_deliveries (deleted_at);
    create index idx_event_deliveries_device_id_key on convoy.event_deliveries (device_id);
    create index idx_event_deliveries_endpoint_id_key on convoy.event_deliveries (endpoint_id);
    create index idx_event_deliveries_event_id_key on convoy.event_deliveries (event_id);
    create index idx_event_deliveries_project_id_endpoint_id on convoy.event_deliveries (project_id, endpoint_id);
    create index idx_event_deliveries_project_id_endpoint_id_status on convoy.event_deliveries (project_id, endpoint_id, status);
    create index idx_event_deliveries_project_id_event_id on convoy.event_deliveries (project_id, event_id);
    create index idx_event_deliveries_project_id_key on convoy.event_deliveries (project_id);
    create index idx_event_deliveries_status on convoy.event_deliveries (status);
    create index idx_event_deliveries_status_key on convoy.event_deliveries (status);

    -- Recreate FK using trigger
    CREATE OR REPLACE TRIGGER event_delivery_fk_check
    BEFORE INSERT ON convoy.delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION enforce_event_delivery_fk();

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select partition_event_deliveries_table();
`

const unPartitionEventDeliveriesTableSQL = `
create or replace function convoy.un_partition_event_deliveries_table() returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of event deliveries table...';

	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.event_deliveries_new;

    -- Create partitioned table
    CREATE TABLE convoy.event_deliveries_new
    (
        id               VARCHAR not null primary key ,
        status           TEXT    not null,
        description      TEXT    not null,
        project_id       VARCHAR not null references convoy.projects,
        endpoint_id      VARCHAR references convoy.endpoints,
        event_id         VARCHAR not null,
        device_id        VARCHAR references convoy.devices,
        subscription_id  VARCHAR not null references convoy.subscriptions,
        metadata         jsonb   not null,
        headers          jsonb,
        attempts         bytea,
        cli_metadata     jsonb,
        created_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        updated_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at       TIMESTAMP WITH TIME ZONE,
        url_query_params VARCHAR,
        idempotency_key  TEXT,
        latency          TEXT,
        event_type       TEXT,
        acknowledged_at  TIMESTAMP WITH TIME ZONE,
        latency_seconds  NUMERIC,
        delivery_mode    convoy.delivery_mode NOT NULL DEFAULT 'at_least_once'
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.event_deliveries_new (
        id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
        attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
        latency_seconds, delivery_mode
    )
    SELECT id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
           attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
           latency_seconds, COALESCE(delivery_mode, 'at_least_once')::convoy.delivery_mode
    FROM convoy.event_deliveries;

    ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT if exists delivery_attempts_event_delivery_id_fkey;
    ALTER TABLE convoy.delivery_attempts
        ADD CONSTRAINT delivery_attempts_event_delivery_id_fkey
            FOREIGN KEY (event_delivery_id) REFERENCES convoy.event_deliveries_new (id);

    ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_old;
    ALTER TABLE convoy.event_deliveries_new RENAME TO event_deliveries;
    DROP TABLE IF EXISTS convoy.event_deliveries_old;

    RAISE NOTICE 'Recreating indexes...';
    create index event_deliveries_event_type on convoy.event_deliveries (event_type);
    create index idx_event_deliveries_created_at_key on convoy.event_deliveries (created_at);
    create index idx_event_deliveries_deleted_at_key on convoy.event_deliveries (deleted_at);
    create index idx_event_deliveries_device_id_key on convoy.event_deliveries (device_id);
    create index idx_event_deliveries_endpoint_id_key on convoy.event_deliveries (endpoint_id);
    create index idx_event_deliveries_event_id_key on convoy.event_deliveries (event_id);
    create index idx_event_deliveries_project_id_endpoint_id on convoy.event_deliveries (project_id, endpoint_id);
    create index idx_event_deliveries_project_id_endpoint_id_status on convoy.event_deliveries (project_id, endpoint_id, status);
    create index idx_event_deliveries_project_id_event_id on convoy.event_deliveries (project_id, event_id);
    create index idx_event_deliveries_project_id_key on convoy.event_deliveries (project_id);
    create index idx_event_deliveries_status on convoy.event_deliveries (status);
    create index idx_event_deliveries_status_key on convoy.event_deliveries (status);

	RAISE NOTICE 'Successfully un-partitioned events table...';
end $$ language plpgsql;
select convoy.un_partition_event_deliveries_table()
`
