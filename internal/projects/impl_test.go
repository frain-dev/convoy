package projects

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

// TestSignatureVersionSerialization tests that signature versions are correctly
// serialized and deserialized, preventing the bug where only UIDs were stored
func TestSignatureVersionSerialization(t *testing.T) {
	tests := []struct {
		name     string
		versions datastore.SignatureVersions
	}{
		{
			name: "single_signature_version",
			versions: []datastore.SignatureVersion{
				{
					UID:       ulid.Make().String(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: time.Now(),
				},
			},
		},
		{
			name: "multiple_signature_versions",
			versions: []datastore.SignatureVersion{
				{
					UID:       ulid.Make().String(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: time.Now(),
				},
				{
					UID:       ulid.Make().String(),
					Hash:      "SHA512",
					Encoding:  datastore.Base64Encoding,
					CreatedAt: time.Now(),
				},
			},
		},
		{
			name:     "empty_signature_versions",
			versions: []datastore.SignatureVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			jsonBytes := signatureVersionsToJSON(tt.versions)
			require.NotNil(t, jsonBytes)

			// Verify it's valid JSON
			var checkArray []map[string]interface{}
			err := json.Unmarshal(jsonBytes, &checkArray)
			require.NoError(t, err)

			// If not empty, verify structure includes all fields, not just UIDs
			if len(tt.versions) > 0 {
				require.Len(t, checkArray, len(tt.versions))

				// Verify first element has all required fields, not just UID
				// Note: JSON marshaling uses lowercase field names from struct tags
				firstElem := checkArray[0]
				require.Contains(t, firstElem, "uid")
				require.Contains(t, firstElem, "hash")
				require.Contains(t, firstElem, "encoding")

				// Critical: Verify we're not just storing UIDs
				require.Equal(t, tt.versions[0].Hash, firstElem["hash"])
				require.Equal(t, string(tt.versions[0].Encoding), firstElem["encoding"])
			}

			// Deserialize
			deserialized := jsonToSignatureVersions(jsonBytes)
			require.Len(t, deserialized, len(tt.versions))

			// Verify all fields match
			for i, version := range tt.versions {
				require.Equal(t, version.UID, deserialized[i].UID)
				require.Equal(t, version.Hash, deserialized[i].Hash)
				require.Equal(t, version.Encoding, deserialized[i].Encoding)
			}
		})
	}
}

// TestSignatureVersionRoundTrip tests complete round-trip through database
func TestSignatureVersionRoundTrip(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Create project with signature versions
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Signature Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config: &datastore.ProjectConfig{
			Signature: &datastore.SignatureConfiguration{
				Header: config.DefaultSignatureHeader,
				Versions: []datastore.SignatureVersion{
					{
						UID:       ulid.Make().String(),
						Hash:      "SHA256",
						Encoding:  datastore.HexEncoding,
						CreatedAt: time.Now(),
					},
					{
						UID:       ulid.Make().String(),
						Hash:      "SHA512",
						Encoding:  datastore.Base64Encoding,
						CreatedAt: time.Now(),
					},
				},
			},
			Strategy: &datastore.StrategyConfiguration{
				Type:       datastore.LinearStrategyProvider,
				Duration:   10,
				RetryCount: 3,
			},
			RateLimit:     &datastore.RateLimitConfiguration{Count: 5000, Duration: 60},
			ReplayAttacks: false,
			MaxIngestSize: 5242880,
		},
	}

	// Create project
	err := service.CreateProject(ctx, project)
	require.NoError(t, err)

	// Fetch project back
	fetched, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)

	// Critical assertions: Verify ALL signature version fields are preserved
	require.NotNil(t, fetched.Config)
	require.NotNil(t, fetched.Config.Signature)
	require.Len(t, fetched.Config.Signature.Versions, 2)

	// Verify first version
	require.Equal(t, project.Config.Signature.Versions[0].UID, fetched.Config.Signature.Versions[0].UID)
	require.Equal(t, project.Config.Signature.Versions[0].Hash, fetched.Config.Signature.Versions[0].Hash)
	require.Equal(t, project.Config.Signature.Versions[0].Encoding, fetched.Config.Signature.Versions[0].Encoding)

	// Verify second version
	require.Equal(t, project.Config.Signature.Versions[1].UID, fetched.Config.Signature.Versions[1].UID)
	require.Equal(t, project.Config.Signature.Versions[1].Hash, fetched.Config.Signature.Versions[1].Hash)
	require.Equal(t, project.Config.Signature.Versions[1].Encoding, fetched.Config.Signature.Versions[1].Encoding)
}

// TestJsonToSignatureVersionsErrorHandling tests error cases
func TestJsonToSignatureVersionsErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectDefault bool
	}{
		{
			name:          "empty_data",
			input:         []byte{},
			expectDefault: true,
		},
		{
			name:          "nil_data",
			input:         nil,
			expectDefault: true,
		},
		{
			name:          "invalid_json",
			input:         []byte(`{"invalid": json`),
			expectDefault: true,
		},
		{
			name:          "array_of_strings_instead_of_objects",
			input:         []byte(`["01KF0ZW15790G4V89X0HPZFHM7", "01KF0ZW15790G4V89X0HPZFHM8"]`),
			expectDefault: true,
		},
		{
			name:          "valid_json",
			input:         []byte(`[{"UID":"123","Hash":"SHA256","Encoding":"hex"}]`),
			expectDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := jsonToSignatureVersions(tt.input)
			require.NotNil(t, result)

			if tt.expectDefault {
				// Should return default signature versions
				require.Len(t, result, 1)
				require.Equal(t, "SHA256", result[0].Hash)
				require.Equal(t, datastore.HexEncoding, result[0].Encoding)
			} else {
				// Should return parsed data
				require.Len(t, result, 1)
				require.Equal(t, "123", result[0].UID)
				require.Equal(t, "SHA256", result[0].Hash)
			}
		})
	}
}

// TestPgTextToSliceEmptyHandling tests the empty string bug fix
func TestPgTextToSliceEmptyHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Text
		expected []string
	}{
		{
			name:     "valid_non_empty",
			input:    pgtype.Text{String: "a,b,c", Valid: true},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "valid_empty_string",
			input:    pgtype.Text{String: "", Valid: true},
			expected: []string{},
		},
		{
			name:     "invalid_null",
			input:    pgtype.Text{String: "", Valid: false},
			expected: []string{},
		},
		{
			name:     "single_value",
			input:    pgtype.Text{String: "single", Valid: true},
			expected: []string{"single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pgTextToSlice(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestSliceToPgTextConversion tests the conversion from slice to pgtype.Text
func TestSliceToPgTextConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected pgtype.Text
	}{
		{
			name:     "non_empty_slice",
			input:    []string{"a", "b", "c"},
			expected: pgtype.Text{String: "a,b,c", Valid: true},
		},
		{
			name:     "empty_slice",
			input:    []string{},
			expected: pgtype.Text{String: "", Valid: false},
		},
		{
			name:     "nil_slice",
			input:    nil,
			expected: pgtype.Text{String: "", Valid: false},
		},
		{
			name:     "single_value",
			input:    []string{"single"},
			expected: pgtype.Text{String: "single", Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sliceToPgText(tt.input)
			require.Equal(t, tt.expected.String, result.String)
			require.Equal(t, tt.expected.Valid, result.Valid)
		})
	}
}

// TestMetaEventsEventTypeRoundTrip tests MetaEventsEventType serialization
func TestMetaEventsEventTypeRoundTrip(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	eventTypes := []string{"endpoint.created", "endpoint.updated", "subscription.created"}

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "MetaEvents Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config: &datastore.ProjectConfig{
			MetaEvent: &datastore.MetaEventConfiguration{
				IsEnabled: true,
				Type:      datastore.HTTPMetaEvent,
				EventType: eventTypes,
				URL:       "https://example.com/webhook",
				Secret:    "test-secret",
			},
			Signature:     getDefaultProjectConfig().Signature,
			Strategy:      &datastore.StrategyConfiguration{Type: datastore.LinearStrategyProvider, Duration: 10, RetryCount: 3},
			RateLimit:     &datastore.RateLimitConfiguration{Count: 5000, Duration: 60},
			ReplayAttacks: false,
			MaxIngestSize: 5242880,
		},
	}

	// Create project
	err := service.CreateProject(ctx, project)
	require.NoError(t, err)

	// Fetch project back
	fetched, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)

	// Verify MetaEvents EventType is preserved correctly
	require.NotNil(t, fetched.Config.MetaEvent)
	require.Len(t, fetched.Config.MetaEvent.EventType, 3)
	// Convert pq.StringArray to []string for comparison
	fetchedEventTypes := []string(fetched.Config.MetaEvent.EventType)
	require.Equal(t, eventTypes, fetchedEventTypes)
	require.Equal(t, "endpoint.created", fetchedEventTypes[0])
	require.Equal(t, "endpoint.updated", fetchedEventTypes[1])
	require.Equal(t, "subscription.created", fetchedEventTypes[2])
}
