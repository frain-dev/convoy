package v20240101

import (
	"context"
	"fmt"
	"time"
)

type CreateEndpointMigration struct{}

func (m *CreateEndpointMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if timeout, ok := d["http_timeout"].(string); ok && timeout != "" {
		seconds, err := transformDurationStringToInt(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid http_timeout format: %w", err)
		}
		d["http_timeout"] = seconds
	}

	if duration, ok := d["rate_limit_duration"].(string); ok && duration != "" {
		seconds, err := transformDurationStringToInt(duration)
		if err != nil {
			return nil, fmt.Errorf("invalid rate_limit_duration format: %w", err)
		}
		d["rate_limit_duration"] = seconds
	}

	if _, ok := d["advanced_signatures"]; !ok {
		d["advanced_signatures"] = false
	}

	return d, nil
}

func (m *CreateEndpointMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if timeout, ok := d["http_timeout"].(float64); ok {
		d["http_timeout"] = transformIntToDurationString(uint64(timeout))
	}

	if duration, ok := d["rate_limit_duration"].(float64); ok {
		d["rate_limit_duration"] = transformIntToDurationString(uint64(duration))
	}

	return d, nil
}

type UpdateEndpointMigration struct{}

func (m *UpdateEndpointMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if timeout, ok := d["http_timeout"].(string); ok && timeout != "" {
		seconds, err := transformDurationStringToInt(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid http_timeout format: %w", err)
		}
		d["http_timeout"] = seconds
	}

	if duration, ok := d["rate_limit_duration"].(string); ok && duration != "" {
		seconds, err := transformDurationStringToInt(duration)
		if err != nil {
			return nil, fmt.Errorf("invalid rate_limit_duration format: %w", err)
		}
		d["rate_limit_duration"] = seconds
	}

	if _, ok := d["advanced_signatures"]; !ok {
		d["advanced_signatures"] = false
	}

	return d, nil
}

func (m *UpdateEndpointMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if timeout, ok := d["http_timeout"].(float64); ok {
		d["http_timeout"] = transformIntToDurationString(uint64(timeout))
	}

	if duration, ok := d["rate_limit_duration"].(float64); ok {
		d["rate_limit_duration"] = transformIntToDurationString(uint64(duration))
	}

	return d, nil
}

type EndpointResponseMigration struct{}

func (m *EndpointResponseMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	return data, nil
}

func (m *EndpointResponseMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if timeout, ok := d["http_timeout"].(float64); ok {
		d["http_timeout"] = transformIntToDurationString(uint64(timeout))
	}

	if duration, ok := d["rate_limit_duration"].(float64); ok {
		d["rate_limit_duration"] = transformIntToDurationString(uint64(duration))
	}

	return d, nil
}

func transformDurationStringToInt(d string) (uint64, error) {
	id, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}
	return uint64(id.Seconds()), nil
}

func transformIntToDurationString(t uint64) string {
	td := time.Duration(t) * time.Second
	return td.String()
}
