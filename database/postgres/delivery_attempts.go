package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
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
	_                          datastore.DeliveryAttemptsRepository = (*deliveryAttemptRepo)(nil)
	ErrDeliveryAttemptNotFound                                      = errors.New("job not found")
)

const (
	creatDeliveryAttempt = `
    INSERT INTO convoy.delivery_attempts (id, url, method, api_version, endpoint_id, event_delivery_id, ip_address, request_http_header, response_http_header, http_status, response_data, error, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13);
    `

	findDeliveryAttempts = `SELECT * FROM convoy.delivery_attempts WHERE event_delivery_id = $1 order by id;`

	findOneDeliveryAttempt = `SELECT * FROM convoy.delivery_attempts WHERE id = $1 and event_delivery_id = $2;`
)

func (d *deliveryAttemptRepo) CreateDeliveryAttempt(ctx context.Context, attempt *datastore.DeliveryAttempt) error {
	result, err := d.db.ExecContext(
		ctx, creatDeliveryAttempt, attempt.UID, attempt.URL, attempt.Method, attempt.APIVersion, attempt.EndpointID,
		attempt.EventDeliveryId, attempt.IPAddress, attempt.RequestHeader, attempt.ResponseHeader, attempt.HttpResponseCode,
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
			return nil, ErrDeliveryAttemptNotFound
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

func (e *deliveryAttemptRepo) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	return exportRecords(ctx, e.db, "convoy.delivery_attempts", projectID, createdAt, w)
}
