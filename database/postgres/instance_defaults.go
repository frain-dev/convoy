package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"github.com/oklog/ulid/v2"
	"time"
)

var (
	ErrKeyCannotBeEmpty          = errors.New("key cannot be empty")
	ErrDefaultValueCannotBeEmpty = errors.New("default_value (plaintext) cannot be empty")
	ErrInvalidIngestRate         = errors.New("invalid ingest rate json value")
	ErrInvalidRetentionPolicy    = errors.New("invalid retention policy json value")
)

type instanceDefaultsRepo struct {
	db database.Database
}

func NewInstanceDefaultsRepo(db database.Database) datastore.InstanceDefaultsRepository {
	return &instanceDefaultsRepo{
		db: db,
	}
}

func (i *instanceDefaultsRepo) Create(ctx context.Context, instanceDefault *datastore.InstanceDefaults) (*datastore.InstanceDefaults, error) {
	err := validate(instanceDefault)
	if err != nil {
		return nil, err
	}

	encryptionPassphrase := instance.GetEncryptionPassphrase()

	instanceDefault.UID = ulid.Make().String()

	query := `
        INSERT INTO convoy.instance_defaults (id, scope_type, key, default_value_cipher, created_at, updated_at)
        VALUES ($1, $2, $3, pgp_sym_encrypt($4::text, $5), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    `
	_, err = i.db.GetDB().ExecContext(ctx, query,
		instanceDefault.UID,
		instanceDefault.ScopeType,
		instanceDefault.Key,
		instanceDefault.DefaultValue,
		encryptionPassphrase,
	)

	if err != nil {
		return nil, err
	}

	d, err := i.FetchByID(ctx, instanceDefault.UID)
	if err != nil {
		return nil, err
	}

	return d, err
}

func validate(instanceDefault *datastore.InstanceDefaults) error {
	validScopeTypes := map[string]bool{
		"organisation": true,
		"project":      true,
	}
	if !validScopeTypes[instanceDefault.ScopeType] {
		return fmt.Errorf("invalid scope_type: %s, must be 'organisation' or 'project'", instanceDefault.ScopeType)
	}

	if instanceDefault.Key == "" {
		return ErrKeyCannotBeEmpty
	}
	validKeys := map[string]bool{
		instance.KeyInstanceIngestRate: true,
		instance.KeyRetentionPolicy:    true,
	}
	if !validKeys[instanceDefault.Key] {
		return fmt.Errorf("invalid key: %s", instanceDefault.Key)
	}

	if instanceDefault.DefaultValue == "" {
		return ErrDefaultValueCannotBeEmpty
	}

	if instanceDefault.Key == instance.KeyInstanceIngestRate {
		var ingestRate instance.IngestRate
		err := json.Unmarshal([]byte(instanceDefault.DefaultValue), &ingestRate)
		if err != nil || ingestRate.Value == 0 {
			return ErrInvalidIngestRate
		}
	} else {
		var retentionPolicy config.RetentionPolicyConfiguration
		err := json.Unmarshal([]byte(instanceDefault.DefaultValue), &retentionPolicy)
		if err != nil {
			return ErrInvalidRetentionPolicy
		}
		_, err = time.ParseDuration(retentionPolicy.Policy)
		if err != nil {
			return ErrInvalidRetentionPolicy
		}
	}

	return nil
}

func (i *instanceDefaultsRepo) Update(ctx context.Context, id string, instanceDefault *datastore.InstanceDefaults) (*datastore.InstanceDefaults, error) {
	err := validate(instanceDefault)
	if err != nil {
		return nil, err
	}
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	query := `
        UPDATE convoy.instance_defaults
        SET scope_type = $1,
            key = $2,
            default_value_cipher = pgp_sym_encrypt($3::text, $4),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $5
    `
	_, err = i.db.GetDB().ExecContext(ctx, query,
		instanceDefault.ScopeType,
		instanceDefault.Key,
		instanceDefault.DefaultValue,
		encryptionPassphrase,
		id,
	)
	if err != nil {
		return nil, err
	}

	d, err := i.FetchByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return d, err
}

func (i *instanceDefaultsRepo) FetchByID(ctx context.Context, id string) (*datastore.InstanceDefaults, error) {
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	query := `
        SELECT id, scope_type, key,
               pgp_sym_decrypt(default_value_cipher::bytea, $1) AS default_value_cipher,
               created_at, updated_at, deleted_at
        FROM convoy.instance_defaults
        WHERE id = $2
    `

	row := i.db.GetDB().QueryRowContext(ctx, query, encryptionPassphrase, id)
	instanceDefault := &datastore.InstanceDefaults{}
	err := row.Scan(
		&instanceDefault.UID,
		&instanceDefault.ScopeType,
		&instanceDefault.Key,
		&instanceDefault.DefaultValue,
		&instanceDefault.CreatedAt,
		&instanceDefault.UpdatedAt,
		&instanceDefault.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrConfigNotFound
		}
		return nil, err
	}
	return instanceDefault, nil
}

func (i *instanceDefaultsRepo) LoadPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.InstanceDefaults, datastore.PaginationData, error) {
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	var query string
	var args []interface{}

	if pageable.PrevCursor != "" {
		query = `
            SELECT id, scope_type, key,
                   pgp_sym_decrypt(default_value_cipher::bytea, $1) AS default_value_cipher,
                   created_at, updated_at, deleted_at
            FROM convoy.instance_defaults
            WHERE id < $2
            ORDER BY id DESC
            LIMIT $3
        `
		args = append(args, encryptionPassphrase, pageable.PrevCursor, pageable.PerPage+1)
	} else if pageable.NextCursor != "" {
		query = `
            SELECT id, scope_type, key,
                   pgp_sym_decrypt(default_value_cipher::bytea, $1) AS default_value_cipher,
                   created_at, updated_at, deleted_at
            FROM convoy.instance_defaults
            WHERE id > $2
            ORDER BY id
            LIMIT $3
        `
		args = append(args, encryptionPassphrase, pageable.NextCursor, pageable.PerPage+1)
	} else {
		query = `
            SELECT id, scope_type, key,
                   pgp_sym_decrypt(default_value_cipher::bytea, $1) AS default_value_cipher,
                   created_at, updated_at, deleted_at
        FROM convoy.instance_defaults
            ORDER BY id
            LIMIT $2
        `
		args = append(args, encryptionPassphrase, pageable.PerPage+1)
	}

	rows, err := i.db.GetDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer rows.Close()

	var instanceDefaults = make([]datastore.InstanceDefaults, 0)
	var rowCount int
	var firstID, lastID string

	for rows.Next() {
		if rowCount == pageable.PerPage {
			break
		}

		instanceDefault := datastore.InstanceDefaults{}
		err := rows.Scan(
			&instanceDefault.UID,
			&instanceDefault.ScopeType,
			&instanceDefault.Key,
			&instanceDefault.DefaultValue,
			&instanceDefault.CreatedAt,
			&instanceDefault.UpdatedAt,
			&instanceDefault.DeletedAt,
		)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		if rowCount == 0 {
			firstID = instanceDefault.UID
		}
		lastID = instanceDefault.UID

		instanceDefaults = append(instanceDefaults, instanceDefault)
		rowCount++
	}

	hasNextPage := rowCount > pageable.PerPage
	hasPreviousPage := pageable.PrevCursor != ""

	paginationData := datastore.PaginationData{
		PrevRowCount:    datastore.PrevRowCount{Count: 0},
		PerPage:         int64(pageable.PerPage),
		HasNextPage:     hasNextPage,
		HasPreviousPage: hasPreviousPage,
		PrevPageCursor:  firstID,
		NextPageCursor:  lastID,
	}

	return instanceDefaults, paginationData, nil
}
