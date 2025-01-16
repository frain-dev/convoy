package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"io"
	"time"
)

type deliveryAttemptRepo struct {
	db database.Database
}

func NewDeliveryAttemptRepo(db database.Database) datastore.DeliveryAttemptsRepository {
	return &deliveryAttemptRepo{db: db}
}

var (
	_ datastore.DeliveryAttemptsRepository = (*deliveryAttemptRepo)(nil)
)

const (
	creatDeliveryAttempt = `
    INSERT INTO convoy.delivery_attempts (id, url, method, api_version, endpoint_id, event_delivery_id, project_id, ip_address, request_http_header, response_http_header, http_status, response_data, error, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);
    `

	softDeleteProjectDeliveryAttempts = `
    UPDATE convoy.delivery_attempts SET deleted_at = NOW() WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL;
    `

	hardDeleteProjectDeliveryAttempts = `
    DELETE FROM convoy.delivery_attempts WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3;
    `

	findDeliveryAttempts = `with att as (SELECT * FROM convoy.delivery_attempts WHERE event_delivery_id = $1 order by created_at desc limit 10) select * from att order by created_at;`

	findOneDeliveryAttempt = `SELECT * FROM convoy.delivery_attempts WHERE id = $1 and event_delivery_id = $2;`
)

func (d *deliveryAttemptRepo) CreateDeliveryAttempt(ctx context.Context, attempt *datastore.DeliveryAttempt) error {
	result, err := d.db.GetDB().ExecContext(
		ctx, creatDeliveryAttempt, attempt.UID, attempt.URL, attempt.Method, attempt.APIVersion, attempt.EndpointID,
		attempt.EventDeliveryId, attempt.ProjectId, attempt.IPAddress, attempt.RequestHeader, attempt.ResponseHeader, attempt.HttpResponseCode,
		attempt.ResponseData, attempt.Error, attempt.Status,
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

func (d *deliveryAttemptRepo) FindDeliveryAttemptById(ctx context.Context, eventDeliveryId string, id string) (*datastore.DeliveryAttempt, error) {
	attempt := &datastore.DeliveryAttempt{}
	err := d.db.GetReadDB().QueryRowxContext(ctx, findOneDeliveryAttempt, id, eventDeliveryId).StructScan(attempt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrDeliveryAttemptNotFound
		}
		return nil, err
	}

	return attempt, nil
}

func (d *deliveryAttemptRepo) FindDeliveryAttempts(ctx context.Context, eventDeliveryId string) ([]datastore.DeliveryAttempt, error) {
	var attempts []datastore.DeliveryAttempt
	rows, err := d.db.GetReadDB().QueryxContext(ctx, findDeliveryAttempts, eventDeliveryId)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var attempt datastore.DeliveryAttempt

		err = rows.StructScan(&attempt)
		if err != nil {
			return nil, err
		}

		(&attempt).ResponseDataString = string(attempt.ResponseData)

		attempts = append(attempts, attempt)
	}

	return attempts, nil
}

func (d *deliveryAttemptRepo) DeleteProjectDeliveriesAttempts(ctx context.Context, projectID string, filter *datastore.DeliveryAttemptsFilter, hardDelete bool) error {
	var result sql.Result
	var err error

	start := time.Unix(filter.CreatedAtStart, 0)
	end := time.Unix(filter.CreatedAtEnd, 0)

	if hardDelete {
		result, err = d.db.GetDB().ExecContext(ctx, hardDeleteProjectDeliveryAttempts, projectID, start, end)
	} else {
		result, err = d.db.GetDB().ExecContext(ctx, softDeleteProjectDeliveryAttempts, projectID, start, end)
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return datastore.ErrDeliveryAttemptsNotDeleted
	}

	return nil
}

func (d *deliveryAttemptRepo) GetFailureAndSuccessCounts(ctx context.Context, lookBackDuration uint64, resetTimes map[string]time.Time) (map[string]circuit_breaker.PollResult, error) {
	resultsMap := map[string]circuit_breaker.PollResult{}

	query := `
		SELECT
            endpoint_id AS key,
            project_id AS tenant_id,
            COUNT(CASE WHEN status = false THEN 1 END) AS failures,
            COUNT(CASE WHEN status = true THEN 1 END) AS successes
        FROM convoy.delivery_attempts da
        JOIN convoy.projects p
            ON da.project_id = p.id
        LEFT JOIN convoy.project_configurations pc
            ON p.project_configuration_id = pc.id
        WHERE da.created_at >= CASE
        WHEN pc.cb_observability_window IS NOT NULL THEN NOW() - MAKE_INTERVAL(mins := pc.cb_observability_window)
            ELSE NOW() - MAKE_INTERVAL(mins := $1)
        END
        group by endpoint_id, project_id;
	`

	rows, err := d.db.GetReadDB().QueryxContext(ctx, query, lookBackDuration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rowValue circuit_breaker.PollResult
		if rowScanErr := rows.StructScan(&rowValue); rowScanErr != nil {
			return nil, rowScanErr
		}
		resultsMap[rowValue.Key] = rowValue
	}

	// this is an n+1 query? yikes
	query2 := `
		SELECT
	        endpoint_id AS key,
            project_id AS tenant_id,
	        COUNT(CASE WHEN status = false THEN 1 END) AS failures,
	        COUNT(CASE WHEN status = true THEN 1 END) AS successes
	    FROM convoy.delivery_attempts
	    WHERE endpoint_id = '%s' AND created_at >= TIMESTAMP '%s' AT TIME ZONE 'UTC'
	    group by endpoint_id, project_id;
	`

	customFormat := "2006-01-02 15:04:05"
	for k, t := range resetTimes {
		// remove the old key so it doesn't pollute the results
		delete(resultsMap, k)
		qq := fmt.Sprintf(query2, k, t.Format(customFormat))

		var rowValue circuit_breaker.PollResult
		err = d.db.GetReadDB().QueryRowxContext(ctx, qq).StructScan(&rowValue)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
		}

		resultsMap[k] = rowValue
	}

	return resultsMap, nil
}

func (d *deliveryAttemptRepo) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	return exportRecords(ctx, d.db.GetReadDB(), "convoy.delivery_attempts", projectID, createdAt, w)
}

func (d *deliveryAttemptRepo) PartitionDeliveryAttemptsTable(ctx context.Context) error {
	_, err := d.db.GetDB().ExecContext(ctx, partitionDeliveryAttemptsTable)
	if err != nil {
		return err
	}

	return nil
}

func (d *deliveryAttemptRepo) UnPartitionDeliveryAttemptsTable(ctx context.Context) error {
	_, err := d.db.GetDB().ExecContext(ctx, unPartitionDeliveryAttemptsTable)
	if err != nil {
		return err
	}

	return nil
}

var partitionDeliveryAttemptsTable = `
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

var unPartitionDeliveryAttemptsTable = `
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
