package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDeviceNotCreated = errors.New("device could not be created")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrDeviceNotUpdated = errors.New("device could not be updated")
	ErrDeviceNotDeleted = errors.New("device could not be deleted")
)

const (
	createDevice = `
	INSERT INTO convoy.devices (id, project_id, endpoint_id, host_name, status, last_seen_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`

	updateDevice = `
	UPDATE convoy.devices SET
	host_name = $4,
	status = $5,
	last_seen_at = $6,
	updated_at = now()
	WHERE id = $1 AND project_id = $2 AND (endpoint_id = $3 OR $3 = '') AND deleted_at IS NULL;
	`
	updateDeviceLastSeen = `
	UPDATE convoy.devices SET
	status = $4,
	last_seen_at = $5,
	updated_at = now()
	WHERE id = $1 AND project_id = $2 AND (endpoint_id = $3 OR $3 = '') AND deleted_at IS NULL;
	`

	deleteDevice = `
	UPDATE convoy.devices SET
	deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND (endpoint_id = $3 OR $3 = '') AND deleted_at IS NULL;
	`

	fetchDeviceById = `
	SELECT * FROM convoy.devices
	WHERE id = $1 AND project_id = $2 AND (endpoint_id = $3 OR $3 = '') AND deleted_at IS NULL;
	`

	fetchDeviceByHostName = `
	SELECT * FROM convoy.devices
	WHERE host_name = $1 AND project_id = $2 AND (endpoint_id = $3 OR $3 = '') AND deleted_at IS NULL;
	`

	fetchDevicesPaginated = `
	SELECT * FROM convoy.devices WHERE deleted_at IS NULL`

	baseDevicesFilter = `
	AND project_id = :project_id 
	AND (endpoint_id = :endpoint_id OR :endpoint_id = '')`

	baseFetchDevicesPagedForward = `
	%s 
	%s 
	AND id <= :cursor 
	GROUP BY id
	ORDER BY id DESC 
	LIMIT :limit
	`

	baseFetchDevicesPagedBackward = `
	WITH devices AS (  
		%s 
		%s 
		AND id >= :cursor 
		GROUP BY id
		ORDER BY id ASC 
		LIMIT :limit
	)

	SELECT * FROM devices ORDER BY id DESC
	`

	countPrevDevices = `
	SELECT count(distinct(id)) as count
	FROM convoy.devices 
	WHERE deleted_at IS NULL
	%s
	AND id > :cursor GROUP BY id ORDER BY id DESC LIMIT 1`
)

type deviceRepo struct {
	db *sqlx.DB
}

func NewDeviceRepo(db database.Database) datastore.DeviceRepository {
	return &deviceRepo{db: db.GetDB()}
}

func (d *deviceRepo) CreateDevice(ctx context.Context, device *datastore.Device) error {
	r, err := d.db.ExecContext(ctx, createDevice,
		device.UID,
		device.ProjectID,
		device.EndpointID,
		device.HostName,
		device.Status,
		device.LastSeenAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrDeviceNotCreated
	}

	return nil
}

func (d *deviceRepo) UpdateDevice(ctx context.Context, device *datastore.Device, endpointID, projectID string) error {
	r, err := d.db.ExecContext(ctx, updateDevice,
		device.UID,
		projectID,
		endpointID,
		device.HostName,
		device.Status,
		device.LastSeenAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrDeviceNotUpdated
	}

	return nil
}

func (d *deviceRepo) UpdateDeviceLastSeen(ctx context.Context, device *datastore.Device, endpointID, projectID string, status datastore.DeviceStatus) error {
	r, err := d.db.ExecContext(ctx, updateDeviceLastSeen,
		device.UID,
		projectID,
		endpointID,
		status,
		device.LastSeenAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrDeviceNotUpdated
	}

	return nil
}

func (d *deviceRepo) DeleteDevice(ctx context.Context, uid string, endpointID, projectID string) error {
	r, err := d.db.ExecContext(ctx, deleteDevice, uid, projectID, endpointID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrDeviceNotDeleted
	}

	return nil
}

func (d *deviceRepo) FetchDeviceByID(ctx context.Context, uid string, endpointID, projectID string) (*datastore.Device, error) {
	device := &datastore.Device{}
	err := d.db.QueryRowxContext(ctx, fetchDeviceById, uid, projectID, endpointID).StructScan(device)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	return device, nil
}

func (d *deviceRepo) FetchDeviceByHostName(ctx context.Context, hostName string, endpointID, projectID string) (*datastore.Device, error) {
	device := &datastore.Device{}
	err := d.db.QueryRowxContext(ctx, fetchDeviceByHostName, hostName, projectID, endpointID).StructScan(device)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	return device, nil
}

func (d *deviceRepo) LoadDevicesPaged(ctx context.Context, projectID string, filter *datastore.ApiKeyFilter, pageable datastore.Pageable) ([]datastore.Device, datastore.PaginationData, error) {
	var query, filterQuery string
	var args []interface{}
	var err error

	arg := map[string]interface{}{
		"project_id":   projectID,
		"endpoint_id":  filter.EndpointID,
		"endpoint_ids": filter.EndpointIDs,
		"limit":        pageable.Limit(),
		"cursor":       pageable.Cursor(),
	}

	if pageable.Direction == datastore.Next {
		query = baseFetchDevicesPagedForward
	} else {
		query = baseFetchDevicesPagedBackward
	}

	filterQuery = baseDevicesFilter
	if len(filter.EndpointIDs) > 0 {
		filterQuery += ` AND endpoint_id IN (:endpoint_ids)`
	}

	query = fmt.Sprintf(query, fetchDevicesPaginated, filterQuery)

	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = d.db.Rebind(query)

	rows, err := d.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var devices []datastore.Device
	for rows.Next() {
		var data DevicePaginated

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		devices = append(devices, data.Device)
	}

	var count datastore.PrevRowCount
	if len(devices) > 0 {
		var countQuery string
		var qargs []interface{}
		first := devices[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevDevices, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = d.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := d.db.QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(devices))
	for i := range devices {
		ids[i] = devices[i].UID
	}

	if len(devices) > pageable.PerPage {
		devices = devices[:len(devices)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return devices, *pagination, nil
}

type DevicePaginated struct {
	Count int
	datastore.Device
}
