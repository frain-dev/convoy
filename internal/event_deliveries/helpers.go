package event_deliveries

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/event_deliveries/repo"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/jackc/pgx/v5/pgtype"
)

// rowToEventDelivery converts various sqlc-generated row types to datastore.EventDelivery
func rowToEventDelivery(row interface{}) (*datastore.EventDelivery, error) {
	switch r := row.(type) {
	case repo.FindEventDeliveryByIDRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			Description:      r.Description,
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			EndpointID:       common.PgTextToString(r.EndpointID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			LatencySeconds:   numericToFloat64(r.LatencySeconds),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Endpoint: &datastore.Endpoint{
				UID:          common.PgTextToString(r.EndpointMetadataID),
				Name:         common.PgTextToString(r.EndpointMetadataName),
				ProjectID:    common.PgTextToString(r.EndpointMetadataProjectID),
				SupportEmail: common.PgTextToString(r.EndpointMetadataSupportEmail),
				Url:          common.PgTextToString(r.EndpointMetadataUrl),
				OwnerID:      common.PgTextToString(r.EndpointMetadataOwnerID),
			},
			Event: &datastore.Event{EventType: datastore.EventType(r.EventMetadataEventType)},
			Device: &datastore.Device{
				UID:      common.PgTextToString(r.DeviceMetadataID),
				Status:   datastore.DeviceStatus(common.PgTextToString(r.DeviceMetadataStatus)),
				HostName: common.PgTextToString(r.DeviceMetadataHostName),
			},
			Source: &datastore.Source{
				UID:  common.PgTextToString(r.SourceMetadataID),
				Name: common.PgTextToString(r.SourceMetadataName),
			},
		}, nil

	case repo.FindEventDeliveryByIDSlimRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			EndpointID:       common.PgTextToString(r.EndpointID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
		}, nil

	case repo.FindEventDeliveriesByIDsRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			Description:      r.Description,
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			EndpointID:       common.PgTextToString(r.EndpointID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
		}, nil

	case repo.FindEventDeliveriesByEventIDRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			Description:      r.Description,
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			EndpointID:       common.PgTextToString(r.EndpointID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
		}, nil

	case repo.FindDiscardedEventDeliveriesRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			Description:      r.Description,
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
		}, nil

	case repo.FindStuckEventDeliveriesByStatusRow:
		return &datastore.EventDelivery{
			UID:       r.ID,
			ProjectID: r.ProjectID,
		}, nil

	case repo.LoadEventDeliveriesPagedRow:
		return &datastore.EventDelivery{
			UID:              r.ID,
			ProjectID:        r.ProjectID,
			EventID:          r.EventID,
			SubscriptionID:   r.SubscriptionID,
			Headers:          parseHeaders(r.Headers),
			DeliveryAttempts: parseDeliveryAttempts(r.Attempts),
			Status:           datastore.EventDeliveryStatus(r.Status),
			Metadata:         jsonbToMetadata(r.Metadata),
			CLIMetadata:      jsonbToCLIMetadata(r.CliMetadata),
			URLQueryParams:   common.PgTextToString(r.UrlQueryParams),
			IdempotencyKey:   common.PgTextToString(r.IdempotencyKey),
			Description:      r.Description,
			EventType:        datastore.EventType(common.PgTextToString(r.EventType)),
			DeviceID:         common.PgTextToString(r.DeviceID),
			EndpointID:       common.PgTextToString(r.EndpointID),
			DeliveryMode:     datastore.DeliveryMode(common.PgTextToString(r.DeliveryMode)),
			LatencySeconds:   numericToFloat64(r.LatencySeconds),
			CreatedAt:        common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:        common.PgTimestamptzToTime(r.UpdatedAt),
			AcknowledgedAt:   common.PgTimestamptzToNullTime(r.AcknowledgedAt),
			Endpoint: &datastore.Endpoint{
				UID:          common.PgTextToString(r.EndpointMetadataID),
				Name:         common.PgTextToString(r.EndpointMetadataName),
				ProjectID:    common.PgTextToString(r.EndpointMetadataProjectID),
				SupportEmail: common.PgTextToString(r.EndpointMetadataSupportEmail),
				Url:          common.PgTextToString(r.EndpointMetadataUrl),
				OwnerID:      common.PgTextToString(r.EndpointMetadataOwnerID),
			},
			Event: &datastore.Event{EventType: datastore.EventType(r.EventMetadataEventType)},
			Device: &datastore.Device{
				UID:      common.PgTextToString(r.DeviceMetadataID),
				Status:   datastore.DeviceStatus(common.PgTextToString(r.DeviceMetadataStatus)),
				HostName: common.PgTextToString(r.DeviceMetadataHostName),
			},
			Source: &datastore.Source{
				UID:             common.PgTextToString(r.SourceMetadataID),
				Name:            common.PgTextToString(r.SourceMetadataName),
				IdempotencyKeys: r.SourceMetadataIdempotencyKeys,
			},
		}, nil

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
