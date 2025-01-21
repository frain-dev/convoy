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
	"strings"
	"time"
)

var (
	ErrKeyCannotBeEmpty         = errors.New("key cannot be empty")
	ErrInvalidBool              = errors.New("invalid bool json value")
	ErrInvalidIngestRate        = errors.New("invalid ingest rate json value")
	ErrInvalidRetentionPolicy   = errors.New("invalid retention policy json value")
	ErrValueCipherCannotBeEmpty = errors.New("value (plaintext) cannot be empty")
	ErrScopeIDCannotBeEmpty     = errors.New("scope_id cannot be empty")
)

type instanceOverridesRepo struct {
	db database.Database
}

func NewInstanceOverridesRepo(db database.Database) datastore.InstanceOverridesRepository {
	return &instanceOverridesRepo{
		db: db,
	}
}

func (i *instanceOverridesRepo) Create(ctx context.Context, instanceOverride *datastore.InstanceOverrides) (*datastore.InstanceOverrides, error) {
	err := i.validateOverride(ctx, instanceOverride)
	if err != nil {
		return nil, err
	}

	encryptionPassphrase := instance.GetEncryptionPassphrase()

	instanceOverride.UID = ulid.Make().String()

	key := encryptionPassphrase + "-" + instanceOverride.UID

	query := `
        INSERT INTO convoy.instance_overrides (id, scope_type, scope_id, key, value_cipher, created_at, updated_at)
        VALUES ($1, $2, $3, $4, pgp_sym_encrypt($5::text, $6), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        ON CONFLICT (scope_type, scope_id, key)
        DO UPDATE SET
            value_cipher = pgp_sym_encrypt($5::text, CONCAT($7::text, '-', convoy.instance_overrides.id)),
            updated_at = CURRENT_TIMESTAMP
        RETURNING id;
    `

	var uid string
	err = i.db.GetDB().QueryRowContext(ctx, query,
		instanceOverride.UID,
		instanceOverride.ScopeType,
		instanceOverride.ScopeID,
		instanceOverride.Key,
		instanceOverride.Value,
		key,
		encryptionPassphrase,
	).Scan(&uid)
	if err != nil {
		return nil, err
	}

	o, err := i.FetchByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	return o, err
}

func (i *instanceOverridesRepo) validateOverride(ctx context.Context, instanceOverride *datastore.InstanceOverrides) error {
	validScopeTypes := map[string]bool{
		instance.OrganisationScope: true,
		instance.ProjectScope:      true,
	}
	if !validScopeTypes[instanceOverride.ScopeType] {
		return fmt.Errorf("invalid scope_type: %s, must be 'organisation' or 'project'", instanceOverride.ScopeType)
	}

	if instanceOverride.Key == "" {
		return ErrKeyCannotBeEmpty
	}
	validKeys := map[string]bool{
		instance.KeyInstanceIngestRate: true,
		instance.KeyRetentionPolicy:    true,
		instance.KeyStaticIP:           true,
		instance.KeyEnterpriseSSO:      true,
	}
	if !validKeys[instanceOverride.Key] {
		return fmt.Errorf("invalid key: %s", instanceOverride.Key)
	}

	if instanceOverride.Value == "" {
		return ErrValueCipherCannotBeEmpty
	}

	if instanceOverride.ScopeID == "" {
		return ErrScopeIDCannotBeEmpty
	}

	if instanceOverride.Key == instance.KeyInstanceIngestRate {
		var ingestRate instance.IngestRate
		err := json.Unmarshal([]byte(instanceOverride.Value), &ingestRate)
		if err != nil || ingestRate.Value == 0 {
			return ErrInvalidIngestRate
		}
	} else if instanceOverride.Key == instance.KeyRetentionPolicy {
		var retentionPolicy config.RetentionPolicyConfiguration
		err := json.Unmarshal([]byte(instanceOverride.Value), &retentionPolicy)
		if err != nil {
			return ErrInvalidRetentionPolicy
		}
		_, err = time.ParseDuration(retentionPolicy.Policy)
		if err != nil {
			return ErrInvalidRetentionPolicy
		}
	} else if instanceOverride.Key == instance.KeyStaticIP || instanceOverride.Key == instance.KeyEnterpriseSSO {
		var boolean instance.Boolean
		err := json.Unmarshal([]byte(instanceOverride.Value), &boolean)
		if err != nil {
			return ErrInvalidBool
		}
	}

	if instanceOverride.ScopeType == instance.OrganisationScope {
		org := &datastore.Organisation{}
		err := i.db.GetDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1", fetchOrganisation), instanceOverride.ScopeID).StructScan(org)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return datastore.ErrOrgNotFound
			}
			return err
		}
	} else {
		project := &datastore.Project{}
		err := i.db.GetDB().GetContext(ctx, project, fetchProjectById, instanceOverride.ScopeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return datastore.ErrProjectNotFound
			}
			return err
		}
	}

	return nil
}

func (i *instanceOverridesRepo) Update(ctx context.Context, id string, instanceOverride *datastore.InstanceOverrides) (*datastore.InstanceOverrides, error) {
	err := i.validateOverride(ctx, instanceOverride)
	if err != nil {
		return nil, err
	}
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	query := `
        UPDATE convoy.instance_overrides
        SET scope_type = $1,
            scope_id = $2,
            key = $3,
            value_cipher = pgp_sym_encrypt($4::text, CONCAT($5::text, '-', id)),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $6
    `
	_, err = i.db.GetDB().ExecContext(ctx, query,
		instanceOverride.ScopeType,
		instanceOverride.ScopeID,
		instanceOverride.Key,
		instanceOverride.Value,
		encryptionPassphrase,
		id,
	)
	if err != nil {
		return nil, err
	}

	o, err := i.FetchByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return o, err
}

func (i *instanceOverridesRepo) FetchByID(ctx context.Context, id string) (*datastore.InstanceOverrides, error) {
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	query := `
        SELECT id, scope_type, scope_id, key,
               pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id)) AS value_cipher,
               created_at, updated_at, deleted_at
        FROM convoy.instance_overrides
        WHERE id = $2
    `

	row := i.db.GetReadDB().QueryRowContext(ctx, query, encryptionPassphrase, id)
	instanceOverride := &datastore.InstanceOverrides{}
	err := row.Scan(
		&instanceOverride.UID,
		&instanceOverride.ScopeType,
		&instanceOverride.ScopeID,
		&instanceOverride.Key,
		&instanceOverride.Value,
		&instanceOverride.CreatedAt,
		&instanceOverride.UpdatedAt,
		&instanceOverride.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrConfigNotFound
		}
		return nil, err
	}
	return instanceOverride, nil
}

func (i *instanceOverridesRepo) LoadPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.InstanceOverrides, datastore.PaginationData, error) {
	encryptionPassphrase := instance.GetEncryptionPassphrase()

	var query string
	var args []interface{}

	if pageable.PrevCursor != "" {
		query = `
            SELECT id, scope_type, scope_id, key,
                   pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id)) AS value_cipher,
                   created_at, updated_at, deleted_at
            FROM convoy.instance_overrides
            WHERE id < $2
            ORDER BY id DESC
            LIMIT $3
        `
		args = append(args, encryptionPassphrase, pageable.PrevCursor, pageable.PerPage+1)
	} else if pageable.NextCursor != "" {
		query = `
            SELECT id, scope_type, scope_id, key,
                   pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id)) AS value_cipher,
                   created_at, updated_at, deleted_at
            FROM convoy.instance_overrides
            WHERE id > $2
            ORDER BY id
            LIMIT $3
        `
		args = append(args, encryptionPassphrase, pageable.NextCursor, pageable.PerPage+1)
	} else {
		query = `
            SELECT id, scope_type, scope_id, key,
                   pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id)) AS value_cipher,
                   created_at, updated_at, deleted_at
            FROM convoy.instance_overrides
            ORDER BY id
            LIMIT $2
        `
		args = append(args, encryptionPassphrase, pageable.PerPage+1)
	}

	rows, err := i.db.GetReadDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer rows.Close()

	var instanceOverrides = make([]datastore.InstanceOverrides, 0)
	var rowCount int
	var firstID, lastID string

	for rows.Next() {
		if rowCount == int(pageable.PerPage) {
			break
		}

		instanceOverride := datastore.InstanceOverrides{}
		err := rows.Scan(
			&instanceOverride.UID,
			&instanceOverride.ScopeType,
			&instanceOverride.ScopeID,
			&instanceOverride.Key,
			&instanceOverride.Value,
			&instanceOverride.CreatedAt,
			&instanceOverride.UpdatedAt,
			&instanceOverride.DeletedAt,
		)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		if rowCount == 0 {
			firstID = instanceOverride.UID
		}
		lastID = instanceOverride.UID

		instanceOverrides = append(instanceOverrides, instanceOverride)
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

	return instanceOverrides, paginationData, nil
}

func (i *instanceOverridesRepo) DeleteUnUpdatedKeys(ctx context.Context, scopeType, scopeID string, keysToUpdate map[string]bool) error {
	var query string
	var args []interface{}

	query = `
        DELETE FROM convoy.instance_overrides
        WHERE scope_type = $1
          AND scope_id = $2`
	args = append(args, scopeType, scopeID)

	if len(keysToUpdate) > 0 {
		query += ` AND key NOT IN (`
		var keyList []string
		for key := range keysToUpdate {
			keyList = append(keyList, fmt.Sprintf("'%s'", key))
		}
		query += fmt.Sprintf("%s)", strings.Join(keyList, ", "))
	}

	_, err := i.db.GetDB().ExecContext(ctx, query, args...)
	return err
}
