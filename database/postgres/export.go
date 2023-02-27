package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	exportRepoQ = `
    SELECT
        to_jsonb(ed) - 'id' || jsonb_build_object('uid', ed.id) AS json_output
    FROM %s AS ed %s;
    `

	count = `
    SELECT COUNT(*) FROM %s %s;
    `

	where = ` WHERE deleted_at IS NULL AND project_id = $1 AND created_at < $2`
)

type exportRepo struct {
	db *sqlx.DB
}

func NewExportRepo(db database.Database) datastore.ExportRepository {
	return &exportRepo{db: db.GetDB()}
}

func (e *exportRepo) ExportRecords(ctx context.Context, tableName, projectID string, createdAt time.Time, dest interface{}) (int64, error) {
	c := &struct {
		Count int64 `db:"count"`
	}{}

	countQuery := fmt.Sprintf(count, tableName, where)
	err := e.db.QueryRowxContext(ctx, countQuery, projectID, createdAt).StructScan(c)
	if err != nil {
		return 0, err
	}

	if c.Count == 0 { // nothing to export
		return 0, nil
	}

	q := fmt.Sprintf(
		exportRepoQ,
		tableName, where,
	)

	return c.Count, e.db.QueryRowxContext(ctx, q, projectID, createdAt).Scan(dest)
}
