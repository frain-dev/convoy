package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

type exportRepo struct {
	db *sqlx.DB
}

func NewExportRepo(db database.Database) datastore.ExportRepository {
	return &exportRepo{db: db.GetDB()}
}

const (
	exportRepoQ = `
		SELECT to_jsonb(ed) - 'id' || jsonb_build_object('uid', ed.id) AS json_output
		FROM %s AS ed %s
		ORDER BY id ASC
		LIMIT $3
		OFFSET $4;
	`

	count = `
		SELECT COUNT(*) FROM %s %s;
	`

	where = ` WHERE deleted_at IS NULL AND project_id = $1 AND created_at < $2`
)

func (e *exportRepo) ExportRecords(ctx context.Context, tableName, projectID string, createdAt time.Time) (json.RawMessage, int64, error) {
	c := &struct {
		Count int64 `db:"count"`
	}{}

	countQuery := fmt.Sprintf(count, tableName, where)
	err := e.db.QueryRowxContext(ctx, countQuery, projectID, createdAt).StructScan(c)
	if err != nil {
		return nil, 0, err
	}

	if c.Count == 0 { // nothing to export
		return nil, 0, nil
	}

	var (
		batchSize  = 2000
		numDocs    int64
		records    []json.RawMessage
		numBatches = int(math.Ceil(float64(c.Count) / float64(batchSize)))
	)

	q := fmt.Sprintf(exportRepoQ, tableName, where)
	for i := 0; i < numBatches; i++ {
		offset := i * batchSize

		rs, n, err := e.querybatch(ctx, q, projectID, createdAt, batchSize, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to remarshal results: %v", err)
		}

		numDocs += n
		records = append(records, rs...)
	}

	v, err := json.Marshal(records)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to remarshal results: %v", err)
	}

	return v, numDocs, nil
}

func (e *exportRepo) querybatch(ctx context.Context, q, projectID string, createdAt time.Time, batchSize, offset int) ([]json.RawMessage, int64, error) {
	var numDocs int64

	// Calling rows.Close() manually in places before we return is important here to prevent
	//  a memory leak, we cannot use defer in a loop because this can fill up the function stack quickly
	rows, err := e.db.QueryxContext(ctx, q, projectID, createdAt, batchSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []json.RawMessage
	for rows.Next() {
		numDocs++
		var record json.RawMessage
		err = rows.Scan(&record)
		if err != nil {
			return nil, 0, err
		}

		records = append(records, record)
	}

	return records, numDocs, nil
}
