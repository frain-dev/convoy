package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/tidwall/gjson"

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
		LIMIT $4
	`

	count = `
		SELECT COUNT(*) FROM %s %s;
	`

	where = ` WHERE deleted_at IS NULL AND project_id = $1 AND created_at < $2 AND (id = $3 OR $3 = '')`
)

// ExportRecords exports the records from the given table and writes them in json format to the passed writer.
// It's the callers responsibility to close the writer.
func (e *exportRepo) ExportRecords(ctx context.Context, tableName, projectID string, createdAt time.Time, w datastore.WriterSyncerCloser) (int64, error) {
	c := &struct {
		Count int64 `db:"count"`
	}{}

	countQuery := fmt.Sprintf(count, tableName, where)
	err := e.db.QueryRowxContext(ctx, countQuery, projectID, createdAt, "").StructScan(c)
	if err != nil {
		return 0, err
	}

	if c.Count == 0 { // nothing to export
		return 0, nil
	}

	var (
		batchSize  = 5000
		numDocs    int64
		numBatches = int(math.Ceil(float64(c.Count) / float64(batchSize)))
	)

	_, err = w.Write([]byte(`[`))
	if err != nil {
		return 0, err
	}
	fmt.Println("numBatches", numBatches)

	q := fmt.Sprintf(exportRepoQ, tableName, where)
	var (
		n      int64
		lastID string
	)

	for i := 0; i < numBatches; i++ {
		fmt.Println("batch ", i)
		offset := i * batchSize

		n, lastID, err = e.querybatch(context.Background(), q, projectID, lastID, createdAt, batchSize, offset, w)
		fmt.Println("querybatch", err)
		if err != nil {
			return 0, fmt.Errorf("failed to query batch %d: %v", i, err)
		}

		numDocs += n
	}

	_, err = w.Write([]byte(`]`))
	if err != nil {
		return 0, err
	}

	return numDocs, nil
}

var commaJSON = []byte(`,`)

func (e *exportRepo) querybatch(ctx context.Context, q, projectID, lastID string, createdAt time.Time, batchSize, offset int, w io.Writer) (int64, string, error) {
	var numDocs int64

	// Calling rows.Close() manually in places before we return is important here to prevent
	//  a memory leak, we cannot use defer in a loop because this can fill up the function stack quickly
	c := time.Now()

	rows, err := e.db.QueryxContext(ctx, q, projectID, createdAt, lastID, batchSize)
	fmt.Println("QueryxContext since", time.Since(c).Seconds())
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()

	var record json.RawMessage
	records := make([]byte, 0, 20000)

	// scan first record and append it without appending comma
	if rows.Next() {
		numDocs++
		err = rows.Scan(&record)
		if err != nil {
			return 0, "", err
		}

		records = append(records, record...)
	}

	i := 0
	// scan remaining records and prefix a comma before writing it
	for rows.Next() {
		numDocs++
		i++
		err = rows.Scan(&record)
		if err != nil {
			return 0, "", err
		}

		records = append(records, append(commaJSON, record...)...)

		if i == 3000 {
			i = 0

			c := time.Now()
			_, err = w.Write(records)
			if err != nil {
				return 0, "", err
			}
			fmt.Println("since", time.Since(c).Seconds())
			records = records[:0]
		}
	}

	if len(records) > 0 {
		c := time.Now()
		_, err = w.Write(records)
		if err != nil {
			return 0, "", err
		}
		fmt.Println("since", time.Since(c).Seconds())
	}

	value := gjson.Get(string(record), "uid")

	return numDocs, value.String(), nil
}
