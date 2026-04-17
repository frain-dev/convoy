package backup_collector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordToJSON(t *testing.T) {
	values := map[string]string{
		"id":         "abc123",
		"project_id": "proj1",
		"event_type": "user.created",
		"headers":    `{"Content-Type":["application/json"]}`,
		"status":     "Success",
	}

	result := recordToJSON("events", values)

	// id renamed to uid
	require.Equal(t, "abc123", result["uid"])
	require.Nil(t, result["id"])

	// plain strings preserved
	require.Equal(t, "proj1", result["project_id"])
	require.Equal(t, "user.created", result["event_type"])
	require.Equal(t, "Success", result["status"])

	// JSONB column parsed as RawMessage
	headers, ok := result["headers"].(json.RawMessage)
	require.True(t, ok, "headers should be json.RawMessage")
	require.JSONEq(t, `{"Content-Type":["application/json"]}`, string(headers))
}

func TestRecordToJSON_EmptyValues(t *testing.T) {
	values := map[string]string{
		"id":               "abc",
		"url_query_params": "",   // empty string — should NOT be treated as JSON
		"metadata":         "",   // empty string
		"headers":          "{}", // valid JSON
	}

	result := recordToJSON("events", values)

	// Empty strings stay as plain strings, not json.RawMessage
	require.Equal(t, "", result["url_query_params"])
	require.Equal(t, "", result["metadata"])

	// Valid JSON still parsed
	_, ok := result["headers"].(json.RawMessage)
	require.True(t, ok)

	// Can marshal without error
	_, err := json.Marshal(result)
	require.NoError(t, err)
}

func TestRecordToJSON_ByteaColumn(t *testing.T) {
	values := map[string]string{
		"id":   "abc",
		"data": `\x7b226e616d65223a2274657374227d`, // hex-encoded bytea
		"raw":  "some raw text",
	}

	result := recordToJSON("events", values)

	// bytea data should be a plain string, not json.RawMessage
	dataVal, ok := result["data"].(string)
	require.True(t, ok, "data should be a plain string")
	require.Contains(t, dataVal, `\x`)

	// raw is text, also plain string
	require.Equal(t, "some raw text", result["raw"])

	// Must marshal without error
	_, err := json.Marshal(result)
	require.NoError(t, err)
}

func TestIsJSONColumn(t *testing.T) {
	// Events
	require.True(t, isJSONColumn("events", "headers"))
	require.True(t, isJSONColumn("events", "metadata"))
	require.True(t, isJSONColumn("events", "url_query_params"))
	require.False(t, isJSONColumn("events", "data"))       // bytea
	require.False(t, isJSONColumn("events", "raw"))        // text
	require.False(t, isJSONColumn("events", "event_type")) // text

	// Event deliveries
	require.True(t, isJSONColumn("event_deliveries", "headers"))
	require.True(t, isJSONColumn("event_deliveries", "metadata"))
	require.True(t, isJSONColumn("event_deliveries", "cli_metadata"))
	require.True(t, isJSONColumn("event_deliveries", "attempts"))
	require.False(t, isJSONColumn("event_deliveries", "status"))

	// Delivery attempts
	require.True(t, isJSONColumn("delivery_attempts", "request_http_header"))
	require.True(t, isJSONColumn("delivery_attempts", "response_http_header"))
	require.False(t, isJSONColumn("delivery_attempts", "url"))

	// Unknown table
	require.False(t, isJSONColumn("unknown_table", "headers"))
}
