package delivery_attempts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/delivery_attempts/repo"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.DeliveryAttemptsRepository at compile time
var _ datastore.DeliveryAttemptsRepository = (*Service)(nil)

func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

func (s *Service) CreateDeliveryAttempt(ctx context.Context, attempt *datastore.DeliveryAttempt) error {
	if attempt == nil {
		return util.NewServiceError(400, errors.New("delivery attempt cannot be nil"))
	}

	// Convert datastore.DeliveryAttempt to SQLc params
	params := repo.CreateDeliveryAttemptParams{
		ID:              attempt.UID,
		Url:             attempt.URL,
		Method:          attempt.Method,
		ApiVersion:      attempt.APIVersion,
		EndpointID:      attempt.EndpointID,
		EventDeliveryID: attempt.EventDeliveryId,
		ProjectID:       attempt.ProjectId,
		IpAddress:       pgtype.Text{String: attempt.IPAddress, Valid: attempt.IPAddress != ""},
		HttpStatus:      pgtype.Text{String: attempt.HttpResponseCode, Valid: attempt.HttpResponseCode != ""},
		Error:           pgtype.Text{String: attempt.Error, Valid: attempt.Error != ""},
		Status:          pgtype.Bool{Bool: attempt.Status, Valid: true},
	}

	// Marshal headers to JSON bytes
	if attempt.RequestHeader != nil {
		requestHeaderBytes, err := json.Marshal(attempt.RequestHeader)
		if err != nil {
			s.logger.WithError(err).Error("failed to marshal request headers")
			return util.NewServiceError(500, fmt.Errorf("failed to marshal request headers: %w", err))
		}
		params.RequestHttpHeader = requestHeaderBytes
	}

	if attempt.ResponseHeader != nil {
		responseHeaderBytes, err := json.Marshal(attempt.ResponseHeader)
		if err != nil {
			s.logger.WithError(err).Error("failed to marshal response headers")
			return util.NewServiceError(500, fmt.Errorf("failed to marshal response headers: %w", err))
		}
		params.ResponseHttpHeader = responseHeaderBytes
	}

	if attempt.ResponseData != nil {
		params.ResponseData = attempt.ResponseData
	}

	err := s.repo.CreateDeliveryAttempt(ctx, params)
	if err != nil {
		s.logger.WithError(err).Error("failed to create delivery attempt")
		return util.NewServiceError(500, fmt.Errorf("failed to create delivery attempt: %w", err))
	}

	return nil
}

func (s *Service) FindDeliveryAttemptById(ctx context.Context, eventDeliveryId, id string) (*datastore.DeliveryAttempt, error) {
	row, err := s.repo.FindDeliveryAttemptById(ctx, repo.FindDeliveryAttemptByIdParams{
		ID:              id,
		EventDeliveryID: eventDeliveryId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrDeliveryAttemptNotFound
		}
		s.logger.WithError(err).Error("failed to find delivery attempt by id")
		return nil, util.NewServiceError(500, fmt.Errorf("failed to find delivery attempt: %w", err))
	}

	return rowToDeliveryAttempt(row)
}

func (s *Service) FindDeliveryAttempts(ctx context.Context, eventDeliveryId string) ([]datastore.DeliveryAttempt, error) {
	rows, err := s.repo.FindDeliveryAttempts(ctx, eventDeliveryId)
	if err != nil {
		s.logger.WithError(err).Error("failed to find delivery attempts")
		return nil, util.NewServiceError(500, fmt.Errorf("failed to find delivery attempts: %w", err))
	}

	attempts := make([]datastore.DeliveryAttempt, 0, len(rows))
	for _, row := range rows {
		attempt, err := rowToDeliveryAttempt(row)
		if err != nil {
			s.logger.WithError(err).Error("failed to convert row to delivery attempt")
			continue // Skip invalid rows
		}
		attempts = append(attempts, *attempt)
	}

	return attempts, nil
}

func (s *Service) DeleteProjectDeliveriesAttempts(ctx context.Context, projectID string, filter *datastore.DeliveryAttemptsFilter, hardDelete bool) error {
	if filter == nil {
		return util.NewServiceError(400, errors.New("filter cannot be nil"))
	}

	start := time.Unix(filter.CreatedAtStart, 0)
	end := time.Unix(filter.CreatedAtEnd, 0)

	var result pgconn.CommandTag
	var err error

	if hardDelete {
		result, err = s.repo.HardDeleteProjectDeliveryAttempts(ctx, repo.HardDeleteProjectDeliveryAttemptsParams{
			ProjectID:      projectID,
			CreatedAtStart: pgtype.Timestamptz{Time: start, Valid: true},
			CreatedAtEnd:   pgtype.Timestamptz{Time: end, Valid: true},
		})
	} else {
		result, err = s.repo.SoftDeleteProjectDeliveryAttempts(ctx, repo.SoftDeleteProjectDeliveryAttemptsParams{
			ProjectID:      projectID,
			CreatedAtStart: pgtype.Timestamptz{Time: start, Valid: true},
			CreatedAtEnd:   pgtype.Timestamptz{Time: end, Valid: true},
		})
	}

	if err != nil {
		s.logger.WithError(err).Error("failed to delete project delivery attempts")
		return util.NewServiceError(500, fmt.Errorf("failed to delete delivery attempts: %w", err))
	}

	if result.RowsAffected() < 1 {
		return datastore.ErrDeliveryAttemptsNotDeleted
	}

	return nil
}

func (s *Service) GetFailureAndSuccessCounts(ctx context.Context, lookBackDuration uint64, resetTimes map[string]time.Time) (map[string]circuit_breaker.PollResult, error) {
	resultsMap := make(map[string]circuit_breaker.PollResult)

	// First, get counts for all endpoints within the lookback duration
	rows, err := s.repo.GetFailureAndSuccessCounts(ctx, int32(lookBackDuration))
	if err != nil {
		s.logger.WithError(err).Error("failed to get failure and success counts")
		return nil, util.NewServiceError(500, fmt.Errorf("failed to get counts: %w", err))
	}

	for _, row := range rows {
		resultsMap[row.Key] = circuit_breaker.PollResult{
			Key:       row.Key,
			TenantId:  row.TenantID,
			Failures:  uint64(row.Failures),
			Successes: uint64(row.Successes),
		}
	}

	// Now handle endpoints with custom reset times
	// For each endpoint with a reset time, get counts from that time onwards
	for endpointID, resetTime := range resetTimes {
		// Remove the old key to avoid pollution
		delete(resultsMap, endpointID)

		row, err := s.repo.GetFailureAndSuccessCountsWithResetTime(ctx, repo.GetFailureAndSuccessCountsWithResetTimeParams{
			EndpointID: endpointID,
			ResetTime:  pgtype.Timestamptz{Time: resetTime, Valid: true},
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// No results for this endpoint, skip
				continue
			}
			s.logger.WithError(err).Error("failed to get counts for endpoint with reset time")
			continue // Continue processing other endpoints
		}

		resultsMap[endpointID] = circuit_breaker.PollResult{
			Key:       row.Key,
			TenantId:  row.TenantID,
			Failures:  uint64(row.Failures),
			Successes: uint64(row.Successes),
		}
	}

	return resultsMap, nil
}

func (s *Service) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	// Export delivery attempts to JSON format for backup/archival purposes
	// This uses batched queries to avoid loading all records into memory at once

	const (
		countQuery = `
			SELECT COUNT(*)
			FROM convoy.delivery_attempts
			WHERE deleted_at IS NULL
			  AND project_id = $1
			  AND created_at < $2
		`

		exportQuery = `
			SELECT TO_JSONB(da) - 'id' || JSONB_BUILD_OBJECT('uid', da.id) AS json_output
			FROM convoy.delivery_attempts AS da
			WHERE deleted_at IS NULL
			  AND project_id = $1
			  AND created_at < $2
			  AND (id > $3 OR $3 = '')
			ORDER BY id ASC
			LIMIT $4
		`
	)

	// Get total count
	var count int64
	err := s.db.QueryRow(ctx, countQuery, projectID, createdAt).Scan(&count)
	if err != nil {
		return 0, util.NewServiceError(500, fmt.Errorf("failed to count records: %w", err))
	}

	if count == 0 {
		return 0, nil
	}

	// Write opening bracket for JSON array
	if _, err := w.Write([]byte(`[`)); err != nil {
		return 0, util.NewServiceError(500, fmt.Errorf("failed to write opening bracket: %w", err))
	}

	var (
		batchSize  = 3000
		numDocs    int64
		lastID     string
		firstBatch = true
	)

	// Process in batches
	for {
		rows, err := s.db.Query(ctx, exportQuery, projectID, createdAt, lastID, batchSize)
		if err != nil {
			return 0, util.NewServiceError(500, fmt.Errorf("failed to query batch: %w", err))
		}

		batchCount := 0
		var record []byte

		for rows.Next() {
			if err := rows.Scan(&record); err != nil {
				rows.Close()
				return 0, util.NewServiceError(500, fmt.Errorf("failed to scan record: %w", err))
			}

			// Add comma before all records except the first
			if !firstBatch || batchCount > 0 {
				if _, err := w.Write([]byte(`,`)); err != nil {
					rows.Close()
					return 0, util.NewServiceError(500, fmt.Errorf("failed to write comma: %w", err))
				}
			}

			if _, err := w.Write(record); err != nil {
				rows.Close()
				return 0, util.NewServiceError(500, fmt.Errorf("failed to write record: %w", err))
			}

			// Extract UID for pagination
			var recordData map[string]interface{}
			if err := json.Unmarshal(record, &recordData); err == nil {
				if uid, ok := recordData["uid"].(string); ok {
					lastID = uid
				}
			}

			batchCount++
			numDocs++
		}

		rows.Close()

		if err := rows.Err(); err != nil {
			return 0, util.NewServiceError(500, fmt.Errorf("error during row iteration: %w", err))
		}

		// If we got fewer records than batch size, we're done
		if batchCount < batchSize {
			break
		}

		firstBatch = false
	}

	// Write closing bracket for JSON array
	if _, err := w.Write([]byte(`]`)); err != nil {
		return 0, util.NewServiceError(500, fmt.Errorf("failed to write closing bracket: %w", err))
	}

	return numDocs, nil
}

func (s *Service) PartitionDeliveryAttemptsTable(ctx context.Context) error {
	// This executes a complex PL/pgSQL function that partitions the table
	// We keep this as raw SQL execution since it's a DDL operation
	const partitionSQL = `
CREATE OR REPLACE FUNCTION partition_delivery_attempts_table()
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned delivery attempts table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.delivery_attempts_new;

    -- Create partitioned table
   create table convoy.delivery_attempts_new
    (
        id                   VARCHAR not null,
        url                  TEXT    not null,
        method               VARCHAR not null,
        api_version          VARCHAR not null,
        project_id           VARCHAR not null references convoy.projects,
        endpoint_id          VARCHAR not null references convoy.endpoints,
        event_delivery_id    VARCHAR not null,
        ip_address           VARCHAR,
        request_http_header  jsonb,
        response_http_header jsonb,
        http_status          VARCHAR,
        response_data        BYTEA,
        error                TEXT,
        status               BOOLEAN,
        created_at           TIMESTAMP WITH TIME ZONE default now() not null,
        updated_at           TIMESTAMP WITH TIME ZONE default now() not null,
        deleted_at           TIMESTAMP WITH TIME ZONE,
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.delivery_attempts
            GROUP BY created_at::DATE, project_id
            order by created_at::DATE
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'delivery_attempts_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.delivery_attempts_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.delivery_attempts_new (
        id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    )
    SELECT id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    FROM convoy.delivery_attempts;

    -- Manage table renaming
    ALTER TABLE convoy.delivery_attempts RENAME TO delivery_attempts_old;
    ALTER TABLE convoy.delivery_attempts_new RENAME TO delivery_attempts;
    DROP TABLE IF EXISTS convoy.delivery_attempts_old;

    RAISE NOTICE 'Recreating indexes...';
    create index idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
    create index idx_delivery_attempts_created_at_id_event_delivery_id
        on convoy.delivery_attempts using brin (created_at, id, project_id, event_delivery_id)
        where (deleted_at IS NULL);
    create index idx_delivery_attempts_event_delivery_id
        on convoy.delivery_attempts (event_delivery_id);
    create index idx_delivery_attempts_event_delivery_id_created_at
        on convoy.delivery_attempts (event_delivery_id, created_at);
    create index idx_delivery_attempts_event_delivery_id_created_at_desc
        on convoy.delivery_attempts (event_delivery_id asc, created_at desc);

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select partition_delivery_attempts_table();
`

	_, err := s.db.Exec(ctx, partitionSQL)
	if err != nil {
		s.logger.WithError(err).Error("failed to partition delivery attempts table")
		return util.NewServiceError(500, fmt.Errorf("failed to partition table: %w", err))
	}

	return nil
}

func (s *Service) UnPartitionDeliveryAttemptsTable(ctx context.Context) error {
	// This executes a complex PL/pgSQL function that unpartitions the table
	const unPartitionSQL = `
create or replace function convoy.un_partition_delivery_attempts_table() returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of delivery attempts table...';

	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.delivery_attempts_new;

    -- Create partitioned table
    create table convoy.delivery_attempts_new
    (
        id                   VARCHAR not null primary key,
        url                  TEXT    not null,
        method               VARCHAR not null,
        api_version          VARCHAR not null,
        project_id           VARCHAR not null references convoy.projects,
        endpoint_id          VARCHAR not null references convoy.endpoints,
        event_delivery_id    VARCHAR not null,
        ip_address           VARCHAR,
        request_http_header  jsonb,
        response_http_header jsonb,
        http_status          VARCHAR,
        response_data        BYTEA,
        error                TEXT,
        status               BOOLEAN,
        created_at           TIMESTAMP WITH TIME ZONE default now() not null,
        updated_at           TIMESTAMP WITH TIME ZONE default now() not null,
        deleted_at           TIMESTAMP WITH TIME ZONE
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.delivery_attempts_new (
        id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    )
    SELECT id, url, method, api_version, project_id, endpoint_id,
           event_delivery_id, ip_address, request_http_header, response_http_header,
           http_status, response_data::bytea, error, status, created_at,
           updated_at, deleted_at
    FROM convoy.delivery_attempts;

    ALTER TABLE convoy.delivery_attempts RENAME TO delivery_attempts_old;
    ALTER TABLE convoy.delivery_attempts_new RENAME TO delivery_attempts;
    DROP TABLE IF EXISTS convoy.delivery_attempts_old;

    RAISE NOTICE 'Recreating indexes...';
	create index idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
    create index idx_delivery_attempts_created_at_id_event_delivery_id
        on convoy.delivery_attempts using brin (created_at, id, project_id, event_delivery_id)
        where (deleted_at IS NULL);
    create index idx_delivery_attempts_event_delivery_id
        on convoy.delivery_attempts (event_delivery_id);
    create index idx_delivery_attempts_event_delivery_id_created_at
        on convoy.delivery_attempts (event_delivery_id, created_at);
    create index idx_delivery_attempts_event_delivery_id_created_at_desc
        on convoy.delivery_attempts (event_delivery_id asc, created_at desc);

	RAISE NOTICE 'Successfully un-partitioned delivery attempts table...';
end $$ language plpgsql;
select convoy.un_partition_delivery_attempts_table()
`

	_, err := s.db.Exec(ctx, unPartitionSQL)
	if err != nil {
		s.logger.WithError(err).Error("failed to unpartition delivery attempts table")
		return util.NewServiceError(500, fmt.Errorf("failed to unpartition table: %w", err))
	}

	return nil
}

// Helper function to convert database row to datastore.DeliveryAttempt
func rowToDeliveryAttempt(row interface{}) (*datastore.DeliveryAttempt, error) {
	var (
		id, url, method, apiVersion, endpointID, eventDeliveryID, projectID string
		ipAddress, httpStatus, errorMsg                                     pgtype.Text
		requestHeader, responseHeader, responseData                         []byte
		status                                                              pgtype.Bool
		createdAt, updatedAt                                                pgtype.Timestamptz
		deletedAt                                                           pgtype.Timestamptz
	)

	// Type switch to handle different row types
	switch r := row.(type) {
	case repo.FindDeliveryAttemptByIdRow:
		id = r.ID
		url = r.Url
		method = r.Method
		apiVersion = r.ApiVersion
		endpointID = r.EndpointID
		eventDeliveryID = r.EventDeliveryID
		projectID = r.ProjectID
		ipAddress = r.IpAddress
		requestHeader = r.RequestHttpHeader
		responseHeader = r.ResponseHttpHeader
		httpStatus = r.HttpStatus
		responseData = r.ResponseData
		errorMsg = r.Error
		status = r.Status
		createdAt = r.CreatedAt
		updatedAt = r.UpdatedAt
		deletedAt = r.DeletedAt
	case repo.FindDeliveryAttemptsRow:
		id = r.ID
		url = r.Url
		method = r.Method
		apiVersion = r.ApiVersion
		endpointID = r.EndpointID
		eventDeliveryID = r.EventDeliveryID
		projectID = r.ProjectID
		ipAddress = r.IpAddress
		requestHeader = r.RequestHttpHeader
		responseHeader = r.ResponseHttpHeader
		httpStatus = r.HttpStatus
		responseData = r.ResponseData
		errorMsg = r.Error
		status = r.Status
		createdAt = r.CreatedAt
		updatedAt = r.UpdatedAt
		deletedAt = r.DeletedAt
	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	// Parse headers
	var reqHeader, respHeader datastore.HttpHeader
	if len(requestHeader) > 0 {
		if err := json.Unmarshal(requestHeader, &reqHeader); err != nil {
			return nil, fmt.Errorf("failed to unmarshal request headers: %w", err)
		}
	}
	if len(responseHeader) > 0 {
		if err := json.Unmarshal(responseHeader, &respHeader); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response headers: %w", err)
		}
	}

	attempt := &datastore.DeliveryAttempt{
		UID:                id,
		URL:                url,
		Method:             method,
		APIVersion:         apiVersion,
		EndpointID:         endpointID,
		EventDeliveryId:    eventDeliveryID,
		ProjectId:          projectID,
		IPAddress:          ipAddress.String,
		RequestHeader:      reqHeader,
		ResponseHeader:     respHeader,
		HttpResponseCode:   httpStatus.String,
		ResponseData:       responseData,
		ResponseDataString: string(responseData),
		Error:              errorMsg.String,
		Status:             status.Bool,
		CreatedAt:          createdAt.Time,
		UpdatedAt:          updatedAt.Time,
	}

	if deletedAt.Valid {
		attempt.DeletedAt.Time = deletedAt.Time
		attempt.DeletedAt.Valid = true
	}

	return attempt, nil
}
