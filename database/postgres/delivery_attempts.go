package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/jmoiron/sqlx"
	"io"
	"time"
)

type deliveryAttemptRepo struct {
	db *sqlx.DB
}

func NewDeliveryAttemptRepo(db database.Database) datastore.DeliveryAttemptsRepository {
	return &deliveryAttemptRepo{
		db: db.GetDB(),
	}
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
	result, err := d.db.ExecContext(
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
	err := d.db.QueryRowxContext(ctx, findOneDeliveryAttempt, id, eventDeliveryId).StructScan(attempt)
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
	rows, err := d.db.QueryxContext(ctx, findDeliveryAttempts, eventDeliveryId)
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
		result, err = d.db.ExecContext(ctx, hardDeleteProjectDeliveryAttempts, projectID, start, end)
	} else {
		result, err = d.db.ExecContext(ctx, softDeleteProjectDeliveryAttempts, projectID, start, end)
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
        FROM convoy.delivery_attempts
        WHERE created_at >= NOW() - MAKE_INTERVAL(mins := $1) 
        group by endpoint_id, project_id;
	`

	rows, err := d.db.QueryxContext(ctx, query, lookBackDuration)
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
		err = d.db.QueryRowxContext(ctx, qq).StructScan(&rowValue)
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
	return exportRecords(ctx, d.db, "convoy.delivery_attempts", projectID, createdAt, w)
}
