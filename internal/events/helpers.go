package events

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

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
			URLPath:          common.PgTextToString(r.UrlPath),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			Status:           datastore.EventStatus(common.PgTextToString(r.Status)),
			Metadata:         common.PgTextToString(r.Metadata),
			FailureReason:    common.PgTextToString(r.FailureReason),
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
			Endpoints:        parseEndpoints(common.PgTextToString(r.Endpoints)),
			ProjectID:        r.ProjectID,
			SourceID:         common.PgTextToString(r.SourceID),
			Headers:          parseHeaders(r.Headers),
			Raw:              r.Raw,
			Data:             r.Data,
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			URLPath:          common.PgTextToString(r.UrlPath),
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

	case repo.FindFirstEventWithIdempotencyKeyRow:
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
			URLPath:          common.PgTextToString(r.UrlPath),
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

	case repo.LoadEventsPagedExistsInnerDescRow:
		return eventFromExistsPagedRow(
			r.ID, r.EventType, r.ProjectID, r.SourceID, r.Endpoints, r.Headers, r.Raw, r.Data,
			r.UrlQueryParams, r.UrlPath, r.IdempotencyKey, r.IsDuplicateEvent, r.Status, r.Metadata,
			r.FailureReason, r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.AcknowledgedAt, r.SourceMetadataID, r.SourceMetadataName,
		)

	case repo.LoadEventsPagedExistsInnerAscRow:
		return eventFromExistsPagedRow(
			r.ID, r.EventType, r.ProjectID, r.SourceID, r.Endpoints, r.Headers, r.Raw, r.Data,
			r.UrlQueryParams, r.UrlPath, r.IdempotencyKey, r.IsDuplicateEvent, r.Status, r.Metadata,
			r.FailureReason, r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.AcknowledgedAt, r.SourceMetadataID, r.SourceMetadataName,
		)

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
			URLPath:          common.PgTextToString(r.UrlPath),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			IsDuplicateEvent: r.IsDuplicateEvent.Bool,
			Status:           datastore.EventStatus(common.PgTextToString(r.Status)),
			Metadata:         common.PgTextToString(r.Metadata),
			FailureReason:    common.PgTextToString(r.FailureReason),
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
// Uses PostgreSQL array format {a,b,c} for compatibility with legacy pq.StringArray data
func endpointsToString(endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	return "{" + strings.Join(endpoints, ",") + "}"
}

// parseEndpoints converts TEXT to []string
// Handles both PostgreSQL array format {a,b,c} and plain comma-separated format
func parseEndpoints(endpointsStr string) []string {
	if endpointsStr == "" {
		return []string{}
	}
	// Handle PostgreSQL array format from pq.StringArray legacy data
	if strings.HasPrefix(endpointsStr, "{") && strings.HasSuffix(endpointsStr, "}") {
		endpointsStr = endpointsStr[1 : len(endpointsStr)-1]
	}
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

func eventFromExistsPagedRow(
	id, eventType, projectID string,
	sourceID, endpoints pgtype.Text,
	headers []byte,
	raw string,
	data []byte,
	urlQueryParams, urlPath, idempotencyKey pgtype.Text,
	isDuplicate pgtype.Bool,
	status, metadata, failureReason pgtype.Text,
	createdAt, updatedAt pgtype.Timestamptz,
	deletedAt, acknowledgedAt pgtype.Timestamptz,
	sourceMetadataID, sourceMetadataName pgtype.Text,
) (*datastore.Event, error) {
	return &datastore.Event{
		UID:              id,
		EventType:        datastore.EventType(eventType),
		ProjectID:        projectID,
		SourceID:         common.PgTextToString(sourceID),
		Endpoints:        parseEndpoints(common.PgTextToString(endpoints)),
		Headers:          parseHeaders(headers),
		Raw:              raw,
		Data:             data,
		URLQueryParams:   common.PgTextToString(urlQueryParams),
		URLPath:          common.PgTextToString(urlPath),
		IdempotencyKey:   common.PgTextToString(idempotencyKey),
		IsDuplicateEvent: isDuplicate.Bool,
		Status:           datastore.EventStatus(common.PgTextToString(status)),
		Metadata:         common.PgTextToString(metadata),
		FailureReason:    common.PgTextToString(failureReason),
		CreatedAt:        common.PgTimestamptzToTime(createdAt),
		UpdatedAt:        common.PgTimestamptzToTime(updatedAt),
		DeletedAt:        common.PgTimestamptzToNullTime(deletedAt),
		AcknowledgedAt:   common.PgTimestamptzToNullTime(acknowledgedAt),
		Source: &datastore.Source{
			UID:  common.PgTextToString(sourceMetadataID),
			Name: common.PgTextToString(sourceMetadataName),
		},
	}, nil
}
