package common

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// Helper functions
// ============================================================================

// isStringEmpty checks if a string is empty after trimming whitespace.
func isStringEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// ============================================================================
// String to pgtype.Text conversions
// ============================================================================

// StringToPgText converts a string to pgtype.Text.
// Empty strings are represented as invalid (NULL in database).
func StringToPgText(s string) pgtype.Text {
	if isStringEmpty(s) {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// StringToPgTextFilter converts a string to pgtype.Text for filtering.
// Unlike StringToPgText, empty strings are still valid (for filter queries).
func StringToPgTextFilter(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// StringPtrToPgText converts a string pointer to pgtype.Text.
// Nil pointers or empty strings are represented as invalid (NULL in database).
func StringPtrToPgText(s *string) pgtype.Text {
	if s == nil || isStringEmpty(*s) {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ============================================================================
// pgtype.Text to String conversions
// ============================================================================

// PgTextToNullString converts pgtype.Text to null.String.
func PgTextToNullString(t pgtype.Text) null.String {
	return null.NewString(t.String, t.Valid)
}

// StringPtrFromPgText converts pgtype.Text to a string pointer.
// Invalid pgtype.Text or empty strings return nil.
func StringPtrFromPgText(t pgtype.Text) *string {
	if !t.Valid || isStringEmpty(t.String) {
		return nil
	}
	s := t.String
	return &s
}

// ============================================================================
// null.String to pgtype.Text conversions
// ============================================================================

// NullStringToPgText converts null.String to pgtype.Text.
func NullStringToPgText(ns null.String) pgtype.Text {
	return pgtype.Text{String: ns.String, Valid: ns.Valid}
}

// ============================================================================
// Time conversions
// ============================================================================

// PgTimestamptzToNullTime converts pgtype.Timestamptz to null.Time.
func PgTimestamptzToNullTime(t pgtype.Timestamptz) null.Time {
	return null.NewTime(t.Time, t.Valid)
}

// NullTimeToPgTimestamptz converts null.Time to pgtype.Timestamptz.
func NullTimeToPgTimestamptz(t null.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.Time, Valid: t.Valid}
}

// ============================================================================
// Boolean conversions
// ============================================================================

// BoolToPgBool converts a bool to pgtype.Bool.
func BoolToPgBool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

// ============================================================================
// Role conversions (for API keys, organisation members, organisation invites)
// ============================================================================

// RoleToParams converts auth.Role to database column parameters.
// Returns (roleType pgtype.Text, roleProject pgtype.Text, roleEndpoint pgtype.Text).
func RoleToParams(role auth.Role) (pgtype.Text, pgtype.Text, pgtype.Text) {
	roleType := pgtype.Text{
		String: string(role.Type),
		Valid:  !isStringEmpty(string(role.Type)),
	}
	roleProject := pgtype.Text{
		String: role.Project,
		Valid:  !isStringEmpty(role.Project),
	}
	roleEndpoint := pgtype.Text{
		String: role.Endpoint,
		Valid:  !isStringEmpty(role.Endpoint),
	}
	return roleType, roleProject, roleEndpoint
}

// ParamsToRole converts database columns to auth.Role.
func ParamsToRole(roleType, roleProject, roleEndpoint string) auth.Role {
	return auth.Role{
		Type:     auth.RoleType(roleType),
		Project:  roleProject,
		Endpoint: roleEndpoint,
	}
}

// ============================================================================
// JSONB conversions (for filters and other JSONB fields)
// ============================================================================

// MToJSONB converts datastore.M to JSONB []byte for PostgreSQL storage.
// Returns empty JSON object "{}" for nil maps.
func MToJSONB(m datastore.M) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

// JSONBToM converts JSONB []byte from PostgreSQL to datastore.M.
// Returns empty map for empty or null JSONB.
func JSONBToM(data []byte) (datastore.M, error) {
	if len(data) == 0 || string(data) == "{}" {
		return datastore.M{}, nil
	}
	var m datastore.M
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// FlattenM flattens a nested datastore.M structure for efficient matching.
// Uses the M.Flatten() method which handles nested structures.
func FlattenM(m datastore.M) (datastore.M, error) {
	if len(m) == 0 {
		return datastore.M{}, nil
	}

	// Use the Flatten method on M
	mCopy := m
	if err := (&mCopy).Flatten(); err != nil {
		return nil, err
	}
	return mCopy, nil
}

// ============================================================================
// pgtype.Text to string conversions
// ============================================================================

// PgTextToString converts pgtype.Text to string.
// Invalid pgtype.Text returns empty string.
func PgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// ============================================================================
// Timestamptz conversions
// ============================================================================

// PgTimestamptzToTime converts pgtype.Timestamptz to time.Time.
// Invalid timestamptz returns zero time.
func PgTimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimeToPgTimestamptz converts time.Time to pgtype.Timestamptz.
// Zero time is represented as invalid (NULL in database).
func TimeToPgTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ============================================================================
// Array conversions
// ============================================================================

// StringsToPgArray converts []string to []string for pgx (identity function for consistency).
func StringsToPgArray(strs []string) []string {
	if strs == nil {
		return []string{}
	}
	return strs
}

// PgArrayToStrings converts []string from pgx to []string (identity function for consistency).
func PgArrayToStrings(arr []string) []string {
	if arr == nil {
		return []string{}
	}
	return arr
}

// ============================================================================
// JSONB conversion helpers (no error return for convenience)
// ============================================================================

// MToPgJSON converts datastore.M (map) to JSONB bytes.
// Returns empty JSON object on error or nil input.
func MToPgJSON(m datastore.M) []byte {
	if m == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// PgJSONToM converts JSONB bytes to datastore.M (map).
// Returns empty map on error or empty input.
func PgJSONToM(data []byte) datastore.M {
	if len(data) == 0 {
		return make(datastore.M)
	}
	var result datastore.M
	if err := json.Unmarshal(data, &result); err != nil {
		return make(datastore.M)
	}
	return result
}

// RetryFilter JSONB conversions
// ============================================================================

// RetryFilterToJSONB converts datastore.RetryFilter to JSONB []byte for PostgreSQL storage.
// Returns empty JSON object "{}" for nil filters.
func RetryFilterToJSONB(filter datastore.RetryFilter) ([]byte, error) {
	if filter == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(filter)
}

// JSONBToRetryFilter converts JSONB []byte from PostgreSQL to datastore.RetryFilter.
// Returns empty filter for empty or null JSONB.
func JSONBToRetryFilter(data []byte) (datastore.RetryFilter, error) {
	if len(data) == 0 || string(data) == "{}" {
		return datastore.RetryFilter{}, nil
	}
	var filter datastore.RetryFilter
	if err := json.Unmarshal(data, &filter); err != nil {
		return nil, err
	}
	return filter, nil
}
