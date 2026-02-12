package common

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// String conversion tests
// ============================================================================

func TestStringToPgText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pgtype.Text
	}{
		{
			name:     "non-empty string",
			input:    "hello",
			expected: pgtype.Text{String: "hello", Valid: true},
		},
		{
			name:     "empty string",
			input:    "",
			expected: pgtype.Text{String: "", Valid: false},
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: pgtype.Text{String: "", Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToPgText(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestStringToPgTextFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pgtype.Text
	}{
		{
			name:     "non-empty string",
			input:    "hello",
			expected: pgtype.Text{String: "hello", Valid: true},
		},
		{
			name:     "empty string is still valid for filters",
			input:    "",
			expected: pgtype.Text{String: "", Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToPgTextFilter(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPgTextToNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Text
		expected null.String
	}{
		{
			name:     "valid text",
			input:    pgtype.Text{String: "hello", Valid: true},
			expected: null.NewString("hello", true),
		},
		{
			name:     "invalid text",
			input:    pgtype.Text{String: "", Valid: false},
			expected: null.NewString("", false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PgTextToNullString(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Time conversion tests
// ============================================================================

func TestPgTimestamptzToNullTime(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		input    pgtype.Timestamptz
		expected null.Time
	}{
		{
			name:     "valid timestamp",
			input:    pgtype.Timestamptz{Time: now, Valid: true},
			expected: null.NewTime(now, true),
		},
		{
			name:     "invalid timestamp",
			input:    pgtype.Timestamptz{Time: time.Time{}, Valid: false},
			expected: null.NewTime(time.Time{}, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PgTimestamptzToNullTime(tt.input)
			require.Equal(t, tt.expected.Valid, result.Valid)
			if tt.expected.Valid {
				require.Equal(t, tt.expected.Time, result.Time)
			}
		})
	}
}

func TestNullTimeToPgTimestamptz(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		input    null.Time
		expected pgtype.Timestamptz
	}{
		{
			name:     "valid time",
			input:    null.NewTime(now, true),
			expected: pgtype.Timestamptz{Time: now, Valid: true},
		},
		{
			name:     "invalid time",
			input:    null.NewTime(time.Time{}, false),
			expected: pgtype.Timestamptz{Time: time.Time{}, Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTimeToPgTimestamptz(tt.input)
			require.Equal(t, tt.expected.Valid, result.Valid)
			if tt.expected.Valid {
				require.Equal(t, tt.expected.Time, result.Time)
			}
		})
	}
}

func TestTimeToPgTimestamptz(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		input    time.Time
		expected pgtype.Timestamptz
	}{
		{
			name:     "valid time",
			input:    now,
			expected: pgtype.Timestamptz{Time: now, Valid: true},
		},
		{
			name:     "zero time becomes invalid (NULL)",
			input:    time.Time{},
			expected: pgtype.Timestamptz{Time: time.Time{}, Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimeToPgTimestamptz(tt.input)
			require.Equal(t, tt.expected.Valid, result.Valid)
			if tt.expected.Valid {
				require.Equal(t, tt.expected.Time, result.Time)
			}
		})
	}
}

// ============================================================================
// Role conversion tests
// ============================================================================

func TestRoleToParams(t *testing.T) {
	role := auth.Role{
		Type:     auth.RoleProjectAdmin,
		Project:  "project-123",
		Endpoint: "endpoint-456",
	}

	roleType, roleProject, roleEndpoint := RoleToParams(role)

	require.True(t, roleType.Valid)
	require.Equal(t, string(auth.RoleProjectAdmin), roleType.String)

	require.True(t, roleProject.Valid)
	require.Equal(t, "project-123", roleProject.String)

	require.True(t, roleEndpoint.Valid)
	require.Equal(t, "endpoint-456", roleEndpoint.String)
}

func TestRoleToParams_EmptyFields(t *testing.T) {
	role := auth.Role{
		Type:     "",
		Project:  "",
		Endpoint: "",
	}

	roleType, roleProject, roleEndpoint := RoleToParams(role)

	require.False(t, roleType.Valid)
	require.False(t, roleProject.Valid)
	require.False(t, roleEndpoint.Valid)
}

func TestParamsToRole(t *testing.T) {
	role := ParamsToRole("admin", "project-123", "endpoint-456")

	require.Equal(t, auth.RoleType("admin"), role.Type)
	require.Equal(t, "project-123", role.Project)
	require.Equal(t, "endpoint-456", role.Endpoint)
}

// ============================================================================
// JSONB conversion tests (datastore.M)
// ============================================================================

func TestMToJSONB(t *testing.T) {
	tests := []struct {
		name        string
		input       datastore.M
		expected    string
		expectError bool
	}{
		{
			name:     "nil map returns empty object",
			input:    nil,
			expected: "{}",
		},
		{
			name:     "empty map",
			input:    datastore.M{},
			expected: "{}",
		},
		{
			name: "map with values",
			input: datastore.M{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MToJSONB(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tt.expected, string(result))
			}
		})
	}
}

func TestJSONBToM(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    datastore.M
		expectError bool
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: datastore.M{},
		},
		{
			name:     "empty object",
			input:    []byte("{}"),
			expected: datastore.M{},
		},
		{
			name:  "valid JSON",
			input: []byte(`{"key":"value"}`),
			expected: datastore.M{
				"key": "value",
			},
		},
		{
			name:        "invalid JSON",
			input:       []byte("invalid"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONBToM(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

// ============================================================================
// RetryFilter JSONB conversion tests
// ============================================================================

func TestRetryFilterToJSONB(t *testing.T) {
	tests := []struct {
		name        string
		input       datastore.RetryFilter
		expected    string
		expectError bool
	}{
		{
			name:     "nil filter returns empty object",
			input:    nil,
			expected: "{}",
		},
		{
			name:     "empty filter",
			input:    datastore.RetryFilter{},
			expected: "{}",
		},
		{
			name: "filter with values",
			input: datastore.RetryFilter{
				"ProjectID": "project-123",
				"Status":    []string{"Failed"},
			},
			expected: `{"ProjectID":"project-123","Status":["Failed"]}`,
		},
		{
			name: "filter with nested values",
			input: datastore.RetryFilter{
				"ProjectID": "project-123",
				"SearchParams": map[string]any{
					"created_at_start": 1704067200,
					"created_at_end":   1704153600,
				},
			},
			expected: `{"ProjectID":"project-123","SearchParams":{"created_at_start":1704067200,"created_at_end":1704153600}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RetryFilterToJSONB(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tt.expected, string(result))
			}
		})
	}
}

func TestJSONBToRetryFilter(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    datastore.RetryFilter
		expectError bool
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: datastore.RetryFilter{},
		},
		{
			name:     "empty object",
			input:    []byte("{}"),
			expected: datastore.RetryFilter{},
		},
		{
			name:  "valid filter JSON",
			input: []byte(`{"ProjectID":"project-123","Status":["Failed"]}`),
			expected: datastore.RetryFilter{
				"ProjectID": "project-123",
				"Status":    []any{"Failed"},
			},
		},
		{
			name:        "invalid JSON",
			input:       []byte("invalid"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONBToRetryFilter(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRetryFilterRoundTrip(t *testing.T) {
	original := datastore.RetryFilter{
		"ProjectID":   "project-123",
		"EndpointIDs": []any{"ep-1", "ep-2"},
		"Status":      []any{"Failed", "Discarded"},
	}

	// Convert to JSONB
	jsonb, err := RetryFilterToJSONB(original)
	require.NoError(t, err)

	// Convert back to RetryFilter
	result, err := JSONBToRetryFilter(jsonb)
	require.NoError(t, err)

	require.Equal(t, original["ProjectID"], result["ProjectID"])
	require.Equal(t, original["EndpointIDs"], result["EndpointIDs"])
	require.Equal(t, original["Status"], result["Status"])
}
