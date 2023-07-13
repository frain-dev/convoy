package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// ExportRecords exports the records from the given table and writes them in json format to the passed writer.
// It's the callers responsibility to close the writer.
func (e *exportRepo) ExportRecords(ctx context.Context, tableName, projectID string, createdAt time.Time, w datastore.WriterSyncerCloser) (int64, error) {
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

	var (
		batchSize  = 1000
		numDocs    int64
		numBatches = int(math.Ceil(float64(c.Count) / float64(batchSize)))
	)

	_, err = w.Write([]byte(`[`))
	if err != nil {
		return 0, err
	}
	fmt.Println("numBatches", numBatches)

	q := fmt.Sprintf(exportRepoQ, tableName, where)
	for i := 0; i < numBatches; i++ {
		fmt.Println("batch ", i)
		offset := i * batchSize

		n, err := e.querybatch(context.Background(), q, projectID, createdAt, batchSize, offset, w)
		fmt.Println("querybatch", err)
		if err != nil {
			return 0, fmt.Errorf("failed to query batch %d: %v", i, err)
		}

		numDocs += n
		err = w.Sync()
		fmt.Println("sync err ", err)
		if err != nil {
			return 0, err
		}
	}

	_, err = w.Write([]byte(`]`))
	if err != nil {
		return 0, err
	}

	return numDocs, nil
}

func (e *exportRepo) querybatch(ctx context.Context, q, projectID string, createdAt time.Time, batchSize, offset int, w io.Writer) (int64, error) {
	var numDocs int64

	// Calling rows.Close() manually in places before we return is important here to prevent
	//  a memory leak, we cannot use defer in a loop because this can fill up the function stack quickly
	rows, err := e.db.QueryxContext(ctx, q, projectID, createdAt, batchSize, offset)
	fmt.Println("111")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	records := make([]json.RawMessage, 0, batchSize)
	var record json.RawMessage
	fmt.Println("imm befrr next")
	for rows.Next() {
		fmt.Println("imm next")
		numDocs++
		err = rows.Scan(&record)
		fmt.Println("NEXT", err)
		if err != nil {
			return 0, err
		}

		records = append(records, record)
	}

	fmt.Println("222")
	m, err := json.Marshal(records)
	if err != nil {
		return 0, fmt.Errorf("failed to remarshal results: %v", err)
	}

	m = bytes.TrimPrefix(m, []byte(`[`))
	m = bytes.TrimSuffix(m, []byte(`]`))

	_, err = w.Write(m)
	if err != nil {
		return 0, err
	}

	return numDocs, nil
}
