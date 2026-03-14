package event_deliveries

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/event_deliveries/repo"
	"github.com/frain-dev/convoy/pkg/httpheader"
)

// eventDeliveryFields contains the common fields extracted from various sqlc row types.
type eventDeliveryFields struct {
	ID             string
	ProjectID      string
	EventID        string
	SubscriptionID string
	Headers        []byte
	Attempts       []byte
	Status         string
	Metadata       []byte
	CliMetadata    []byte
	UrlQueryParams pgtype.Text
	IdempotencyKey pgtype.Text
	Description    string
	EventType      pgtype.Text
	DeviceID       pgtype.Text
	EndpointID     pgtype.Text
	DeliveryMode   pgtype.Text
	LatencySeconds pgtype.Numeric
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
	AcknowledgedAt pgtype.Timestamptz
}

// buildEventDelivery constructs a datastore.EventDelivery from common fields.
func buildEventDelivery(f eventDeliveryFields) *datastore.EventDelivery {
	return &datastore.EventDelivery{
		UID:              f.ID,
		ProjectID:        f.ProjectID,
		EventID:          f.EventID,
		SubscriptionID:   f.SubscriptionID,
		Headers:          parseHeaders(f.Headers),
		DeliveryAttempts: parseDeliveryAttempts(f.Attempts),
		Status:           datastore.EventDeliveryStatus(f.Status),
		Metadata:         jsonbToMetadata(f.Metadata),
		CLIMetadata:      jsonbToCLIMetadata(f.CliMetadata),
		URLQueryParams:   common.PgTextToString(f.UrlQueryParams),
		IdempotencyKey:   common.PgTextToString(f.IdempotencyKey),
		Description:      f.Description,
		EventType:        datastore.EventType(common.PgTextToString(f.EventType)),
		DeviceID:         common.PgTextToString(f.DeviceID),
		EndpointID:       common.PgTextToString(f.EndpointID),
		DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(f.DeliveryMode)),
		LatencySeconds:   numericToFloat64(f.LatencySeconds),
		CreatedAt:        common.PgTimestamptzToTime(f.CreatedAt),
		UpdatedAt:        common.PgTimestamptzToTime(f.UpdatedAt),
		AcknowledgedAt:   common.PgTimestamptzToNullTime(f.AcknowledgedAt),
	}
}

// joinedMetadata holds metadata from JOINed tables (endpoint, event, device, source).
type joinedMetadata struct {
	EndpointID            pgtype.Text
	EndpointName          pgtype.Text
	EndpointProjectID     pgtype.Text
	EndpointSupportEmail  pgtype.Text
	EndpointUrl           pgtype.Text
	EndpointOwnerID       pgtype.Text
	EventID               string
	EventType             string
	DeviceID              pgtype.Text
	DeviceStatus          pgtype.Text
	DeviceHostName        pgtype.Text
	SourceID              pgtype.Text
	SourceName            pgtype.Text
	SourceIdempotencyKeys []string
}

// applyJoinedMetadata populates the joined entity fields on an EventDelivery.
func applyJoinedMetadata(d *datastore.EventDelivery, m joinedMetadata) {
	d.Endpoint = &datastore.Endpoint{
		UID:          common.PgTextToString(m.EndpointID),
		Name:         common.PgTextToString(m.EndpointName),
		ProjectID:    common.PgTextToString(m.EndpointProjectID),
		SupportEmail: common.PgTextToString(m.EndpointSupportEmail),
		Url:          common.PgTextToString(m.EndpointUrl),
		OwnerID:      common.PgTextToString(m.EndpointOwnerID),
	}
	d.Event = &datastore.Event{EventType: datastore.EventType(m.EventType)}
	d.Device = &datastore.Device{
		UID:      common.PgTextToString(m.DeviceID),
		Status:   datastore.DeviceStatus(common.PgTextToString(m.DeviceStatus)),
		HostName: common.PgTextToString(m.DeviceHostName),
	}
	d.Source = &datastore.Source{
		UID:             common.PgTextToString(m.SourceID),
		Name:            common.PgTextToString(m.SourceName),
		IdempotencyKeys: m.SourceIdempotencyKeys,
	}
}

// rowToEventDelivery converts various sqlc-generated row types to datastore.EventDelivery
func rowToEventDelivery(row interface{}) (*datastore.EventDelivery, error) {
	switch r := row.(type) {
	case repo.FindEventDeliveryByIDRow:
		d := buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			Description: r.Description, EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode, LatencySeconds: r.LatencySeconds,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		})
		applyJoinedMetadata(d, joinedMetadata{
			EndpointID: r.EndpointMetadataID, EndpointName: r.EndpointMetadataName,
			EndpointProjectID: r.EndpointMetadataProjectID, EndpointSupportEmail: r.EndpointMetadataSupportEmail,
			EndpointUrl: r.EndpointMetadataUrl, EndpointOwnerID: r.EndpointMetadataOwnerID,
			EventID: r.EventMetadataID, EventType: r.EventMetadataEventType,
			DeviceID: r.DeviceMetadataID, DeviceStatus: r.DeviceMetadataStatus, DeviceHostName: r.DeviceMetadataHostName,
			SourceID: r.SourceMetadataID, SourceName: r.SourceMetadataName,
		})
		return d, nil

	case repo.FindEventDeliveryByIDSlimRow:
		return buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode,
			CreatedAt:    r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		}), nil

	case repo.FindEventDeliveriesByIDsRow:
		return buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			Description: r.Description, EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode,
			CreatedAt:    r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		}), nil

	case repo.FindEventDeliveriesByEventIDRow:
		return buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			Description: r.Description, EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode,
			CreatedAt:    r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		}), nil

	case repo.FindDiscardedEventDeliveriesRow:
		return buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			Description: r.Description, EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode,
			CreatedAt:    r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		}), nil

	case repo.FindStuckEventDeliveriesByStatusRow:
		return &datastore.EventDelivery{
			UID:       r.ID,
			ProjectID: r.ProjectID,
		}, nil

	case repo.LoadEventDeliveriesPagedRow:
		d := buildEventDelivery(eventDeliveryFields{
			ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, SubscriptionID: r.SubscriptionID,
			Headers: r.Headers, Attempts: r.Attempts, Status: r.Status, Metadata: r.Metadata,
			CliMetadata: r.CliMetadata, UrlQueryParams: r.UrlQueryParams, IdempotencyKey: r.IdempotencyKey,
			Description: r.Description, EventType: r.EventType, DeviceID: r.DeviceID, EndpointID: r.EndpointID,
			DeliveryMode: r.DeliveryMode, LatencySeconds: r.LatencySeconds,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, AcknowledgedAt: r.AcknowledgedAt,
		})
		applyJoinedMetadata(d, joinedMetadata{
			EndpointID: r.EndpointMetadataID, EndpointName: r.EndpointMetadataName,
			EndpointProjectID: r.EndpointMetadataProjectID, EndpointSupportEmail: r.EndpointMetadataSupportEmail,
			EndpointUrl: r.EndpointMetadataUrl, EndpointOwnerID: r.EndpointMetadataOwnerID,
			EventID: r.EventMetadataID, EventType: r.EventMetadataEventType,
			DeviceID: r.DeviceMetadataID, DeviceStatus: r.DeviceMetadataStatus, DeviceHostName: r.DeviceMetadataHostName,
			SourceID: r.SourceMetadataID, SourceName: r.SourceMetadataName,
			SourceIdempotencyKeys: r.SourceMetadataIdempotencyKeys,
		})
		return d, nil

	default:
		return nil, errors.New("unsupported row type")
	}
}

// headersToJSONB converts httpheader.HTTPHeader to JSONB bytes
func headersToJSONB(headers httpheader.HTTPHeader) []byte {
	if headers == nil {
		headers = httpheader.HTTPHeader{}
	}
	data, err := json.Marshal(headers)
	if err != nil {
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
	if err := json.Unmarshal(data, &headers); err != nil {
		return httpheader.HTTPHeader{}
	}
	return headers
}

// metadataToJSONB converts *Metadata to JSONB bytes
func metadataToJSONB(m *datastore.Metadata) []byte {
	if m == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// jsonbToMetadata converts JSONB bytes to *Metadata
func jsonbToMetadata(data []byte) *datastore.Metadata {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var m datastore.Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

// cliMetadataToJSONB converts *CLIMetadata to JSONB bytes
func cliMetadataToJSONB(m *datastore.CLIMetadata) []byte {
	if m == nil {
		return nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return data
}

// jsonbToCLIMetadata converts JSONB bytes to *CLIMetadata
func jsonbToCLIMetadata(data []byte) *datastore.CLIMetadata {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var m datastore.CLIMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

// parseDeliveryAttempts converts bytes to DeliveryAttempts
func parseDeliveryAttempts(data []byte) datastore.DeliveryAttempts {
	if len(data) == 0 {
		return datastore.DeliveryAttempts{}
	}
	var attempts datastore.DeliveryAttempts
	if err := json.Unmarshal(data, &attempts); err != nil {
		return datastore.DeliveryAttempts{}
	}
	return attempts
}

// numericToFloat64 converts pgtype.Numeric to float64
func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}

// getCreatedDateFilter converts Unix timestamps to time.Time
func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	if startDate == 0 && endDate == 0 {
		return time.Unix(0, 0), time.Now()
	}
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}

const minLen = 30

func padIntervals(intervals []datastore.EventInterval, duration time.Duration, period datastore.Period) ([]datastore.EventInterval, error) {
	var err error
	var format string

	switch period {
	case datastore.Daily:
		format = "2006-01-02"
	case datastore.Weekly:
		format = "2006-01-02"
	case datastore.Monthly:
		format = "2006-01"
	case datastore.Yearly:
		format = "2006"
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}

	start := time.Now()
	if len(intervals) > 0 {
		start, err = time.Parse(format, intervals[0].Data.Time)
		if err != nil {
			return nil, err
		}
		start = start.Add(-duration)
	}

	numPadding := minLen - len(intervals)
	paddedIntervals := make([]datastore.EventInterval, numPadding, numPadding+len(intervals))
	for i := numPadding; i > 0; i-- {
		paddedIntervals[i-1] = datastore.EventInterval{
			Data: datastore.EventIntervalData{
				Interval: 0,
				Time:     start.Format(format),
			},
			Count: 0,
		}
		start = start.Add(-duration)
	}

	paddedIntervals = append(paddedIntervals, intervals...)
	return paddedIntervals, nil
}
