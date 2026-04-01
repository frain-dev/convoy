package backup_collector

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// tableToKeySegment maps WAL table names to blob key path segments.
var tableToKeySegment = map[string]string{
	"events":            "events",
	"event_deliveries":  "eventdeliveries",
	"delivery_attempts": "deliveryattempts",
}

// flushLoop runs on a ticker, swapping the buffer and uploading to blob storage.
func (c *BackupCollector) flushLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush on shutdown
			c.doFlush(ctx)
			return
		case <-ticker.C:
			c.doFlush(ctx)
		}
	}
}

func (c *BackupCollector) doFlush(ctx context.Context) {
	records, swapLSN := c.buffer.Swap()
	if len(records) == 0 {
		return
	}

	total := 0
	for _, entries := range records {
		total += len(entries)
	}
	c.logger.Info(fmt.Sprintf("flushing %d records across %d tables (LSN: %s)", total, len(records), swapLSN))

	allOK := true
	for tableName, entries := range records {
		if err := c.flushTable(ctx, tableName, entries); err != nil {
			c.logger.Error(fmt.Sprintf("flush failed for %s: %v", tableName, err))
			allOK = false
		}
	}

	if allOK && swapLSN > 0 {
		c.flushedLSN = swapLSN
		c.logger.Info(fmt.Sprintf("advanced flushed LSN to %s", swapLSN))
	}
}

func (c *BackupCollector) flushTable(ctx context.Context, tableName string, entries []BufferEntry) error {
	if len(entries) == 0 {
		return nil
	}

	segment, ok := tableToKeySegment[tableName]
	if !ok {
		return fmt.Errorf("unknown table: %s", tableName)
	}

	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	ts := now.Format(time.RFC3339)
	blobKey := fmt.Sprintf("backup/%s/%s/%s.jsonl.gz", date, segment, ts)

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		var writeErr error
		gzw := gzip.NewWriter(pw)
		enc := json.NewEncoder(gzw)

		for _, entry := range entries {
			record := recordToJSON(tableName, entry.Values)
			if writeErr = enc.Encode(record); writeErr != nil {
				break
			}
		}

		// Always close gzip first to flush the trailer, then close the pipe
		if closeErr := gzw.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			pw.CloseWithError(writeErr)
		} else {
			pw.Close()
		}
		errCh <- writeErr
	}()

	// Wait for goroutine to finish FIRST by reading errCh,
	// but Upload blocks on pr — so we must read both.
	// Upload returns when pw is closed (by goroutine).
	uploadErr := c.store.Upload(ctx, blobKey, pr)
	encodeErr := <-errCh

	if encodeErr != nil {
		return fmt.Errorf("encode: %w", encodeErr)
	}
	if uploadErr != nil {
		return fmt.Errorf("upload: %w", uploadErr)
	}

	c.logger.Info(fmt.Sprintf("uploaded %d records to %s", len(entries), blobKey))
	return nil
}

// recordToJSON converts WAL column values to a JSON-compatible map.
// Renames "id" to "uid" to match the existing export format.
func recordToJSON(tableName string, values map[string]string) map[string]any {
	result := make(map[string]any, len(values))

	for k, v := range values {
		if k == "id" {
			result["uid"] = v
			continue
		}

		if isJSONColumn(tableName, k) && len(v) > 0 && (v[0] == '{' || v[0] == '[' || v[0] == '"') {
			result[k] = json.RawMessage(v)
			continue
		}

		result[k] = v
	}

	return result
}

// isJSONColumn returns true if the column stores JSON/JSONB data.
func isJSONColumn(tableName, column string) bool {
	jsonColumns := map[string]map[string]bool{
		"events": {
			"headers": true,
			// "data" is bytea, not jsonb — WAL sends it as hex (\x...)
			// "raw" is text, not jsonb
			"url_query_params": true,
			"metadata":         true,
		},
		"event_deliveries": {
			"headers":      true,
			"metadata":     true,
			"cli_metadata": true,
			"attempts":     true,
		},
		"delivery_attempts": {
			"request_http_header":  true,
			"response_http_header": true,
		},
	}

	cols, ok := jsonColumns[tableName]
	if !ok {
		return false
	}
	return cols[column]
}
