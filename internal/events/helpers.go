package events

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/events/repo"
	"github.com/frain-dev/convoy/pkg/httpheader"
)

// rowToEvent converts various sqlc-generated row types to datastore.Event
func rowToEvent(row interface{}) (*datastore.Event, error) {
	switch r := row.(type) {
	case repo.FindEventByIDRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			Endpoints:        parseEndpoints(r.Endpoints),
			ProjectID:        r.ProjectID,
			SourceID:         r.SourceID,
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   r.URLQueryParams,
			IdempotencyKey:   r.IdempotencyKey,
			IsDuplicateEvent: r.IsDuplicateEvent,
			Status:           datastore.EventStatus(r.Status),
			Metadata:         common.PgTextToNullString(r.Metadata).String,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  r.SourceMetadataID,
				Name: r.SourceMetadataName,
			},
		}, nil

	case repo.FindEventsByIDsRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         r.SourceID,
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   r.URLQueryParams,
			IdempotencyKey:   r.IdempotencyKey,
			IsDuplicateEvent: r.IsDuplicateEvent,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  r.SourceMetadataID,
				Name: r.SourceMetadataName,
			},
		}, nil

	case repo.LoadEventsPagedExistsRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         r.SourceID,
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   r.URLQueryParams,
			IdempotencyKey:   r.IdempotencyKey,
			IsDuplicateEvent: r.IsDuplicateEvent,
			Status:           datastore.EventStatus(r.Status),
			Metadata:         common.PgTextToNullString(r.Metadata).String,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  r.SourceMetadataID,
				Name: r.SourceMetadataName,
			},
		}, nil

	case repo.LoadEventsPagedSearchRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         r.SourceID,
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   r.URLQueryParams,
			IdempotencyKey:   r.IdempotencyKey,
			IsDuplicateEvent: r.IsDuplicateEvent,
			Status:           datastore.EventStatus(r.Status),
			Metadata:         r.Metadata,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  r.SourceMetadataID,
				Name: r.SourceMetadataName,
			},
		}, nil

	default:
		return nil, errors.New("unsupported row type")
	}
}

// endpointsToString converts []string to TEXT for storage
// The legacy implementation stores it as a TEXT field (not JSONB array)
func endpointsToString(endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	// Match legacy format: stored as comma-separated or the actual database format
	// The database column is TEXT, matching pq.StringArray in sqlx
	return strings.Join(endpoints, ",")
}

// parseEndpoints converts TEXT to []string
func parseEndpoints(endpointsStr string) []string {
	if endpointsStr == "" {
		return []string{}
	}
	// Handle comma-separated format
	return strings.Split(endpointsStr, ",")
}

// headersToJSONB converts httpheader.HTTPHeader to JSONB bytes
func headersToJSONB(headers httpheader.HTTPHeader) []byte {
	if headers == nil {
		headers = httpheader.HTTPHeader{}
	}
	data, err := json.Marshal(headers)
	if err != nil {
		// Should not happen for valid headers
		return []byte("{}")
	}
	return data
}

// parseHeaders converts JSONB bytes to httpheader.HTTPHeader
func parseHeaders(data []byte) httpheader.HTTPHeader {
	if len(data) == 0 {
		return httpheader.HTTPHeader{}
	}

	var headers httpheader.HTTPHeader
	err := json.Unmarshal(data, &headers)
	if err != nil {
		return httpheader.HTTPHeader{}
	}

	return headers
}
