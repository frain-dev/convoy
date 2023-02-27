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
        json_build_object('uid', id) ||
        (SELECT to_jsonb(t) - 'id' FROM (SELECT * FROM %s %s) t) FROM %s %s;
    `

	count = `
    SELECT COUNT(*) FROM %s %s;
    `

	where = ` WHERE deleted_at IS NULL AND project_id = $1 AMD created_at <= $2`
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

	q := fmt.Sprintf(
		exportRepoQ,
		tableName, where,
		tableName, where,
	)

	return c.Count, e.db.QueryRowxContext(ctx, q, projectID, createdAt).Scan(dest)
}
