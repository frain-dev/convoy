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
			Endpoints:        parseEndpoints(common.PgTextToString(r.Endpoints)),
			ProjectID:        r.ProjectID,
			SourceID:         common.PgTextToString(r.SourceID),
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			Status:           datastore.EventStatus(common.PgTextToString(r.Status)),
			Metadata:         common.PgTextToString(r.Metadata),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  common.PgTextToString(r.SourceMetadataID),
				Name: common.PgTextToString(r.SourceMetadataName),
			},
		}, nil

	case repo.FindEventsByIDsRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         common.PgTextToString(r.SourceID),
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  common.PgTextToString(r.SourceMetadataID),
				Name: common.PgTextToString(r.SourceMetadataName),
			},
		}, nil

	case repo.LoadEventsPagedExistsRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         common.PgTextToString(r.SourceID),
			Endpoints:        parseEndpoints(common.PgTextToString(r.Endpoints)),
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			Status:           datastore.EventStatus(common.PgTextToString(r.Status)),
			Metadata:         common.PgTextToString(r.Metadata),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  common.PgTextToString(r.SourceMetadataID),
				Name: common.PgTextToString(r.SourceMetadataName),
			},
		}, nil

	case repo.LoadEventsPagedSearchRow:
		return &datastore.Event{
			UID:              r.ID,
			EventType:        datastore.EventType(r.EventType),
			ProjectID:        r.ProjectID,
			SourceID:         common.PgTextToString(r.SourceID),
			Endpoints:        parseEndpoints(common.PgTextToString(r.Endpoints)),
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			Status:           datastore.EventStatus(common.PgTextToString(r.Status)),
			Metadata:         common.PgTextToString(r.Metadata),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:        common.PgTimestamptzToNullTime(r.DeletedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Source: &datastore.Source{
				UID:  common.PgTextToString(r.SourceMetadataID),
				Name: common.PgTextToString(r.SourceMetadataName),
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
