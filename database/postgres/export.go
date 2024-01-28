package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/tidwall/gjson"

	"github.com/jmoiron/sqlx"
)

const (
	exportRepoQ = `
		SELECT TO_JSONB(ed) - 'id' || JSONB_BUILD_OBJECT('uid', ed.id) AS json_output
		FROM %s AS ed %s
		ORDER BY id ASC
		LIMIT $4
	`

	count = `
		SELECT COUNT(*) FROM %s %s
	`

	where = ` WHERE deleted_at IS NULL AND project_id = $1 AND created_at < $2 AND (id > $3 OR $3 = '')`
)

// ExportRecords exports the records from the given table and writes them in json format to the passed writer.
// It's the caller's responsibility to close the writer.
func exportRecords(ctx context.Context, db *sqlx.DB, tableName, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	c := &struct {
		Count int64 `db:"count"`
	}{}

	countQuery := fmt.Sprintf(count, tableName, where)
	err := db.QueryRowxContext(ctx, countQuery, projectID, createdAt, "").StructScan(c)
	if err != nil {
		return 0, err
	}

	if c.Count == 0 { // nothing to export
		return 0, nil
	}

	var (
		batchSize  = 3000
		numDocs    int64
		numBatches = int(math.Ceil(float64(c.Count) / float64(batchSize)))
	)

	_, err = w.Write([]byte(`[`))
	if err != nil {
		return 0, err
	}

	q := fmt.Sprintf(exportRepoQ, tableName, where)
	var (
		n      int64
		lastID string
	)

	for i := 0; i < numBatches; i++ {
		n, lastID, err = querybatch(ctx, db, q, projectID, lastID, createdAt, batchSize, w)
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

func querybatch(ctx context.Context, db *sqlx.DB, q, projectID, lastID string, createdAt time.Time, batchSize int, w io.Writer) (int64, string, error) {
	var numDocs int64

	// Calling rows.Close() manually in places before we return is important here to prevent
	//  a memory leak, we cannot use defer in a loop because this can fill up the function stack quickly
	rows, err := db.QueryxContext(ctx, q, projectID, createdAt, lastID, batchSize)
	if err != nil {
		return 0, "", err
	}
	defer closeWithError(rows)

	var record json.RawMessage
	records := make([]byte, 0, 1000)

	// scan the first record and append it without appending a comma
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

		// after gathering 1k records, write records to file
		if i == 100 {
			i = 0

			_, err = w.Write(records)
			if err != nil {
				return 0, "", err
			}
			records = records[:0] // reset records slice it to length 0, so we can reuse the allocated memory
		}
	}

	// check for any unwritten records
	if len(records) > 0 {
		_, err = w.Write(records)
		if err != nil {
			return 0, "", err
		}
	}

	value := gjson.Get(string(record), "uid") // get the id of the last record, we use it for pagination

	return numDocs, value.String(), nil
}
