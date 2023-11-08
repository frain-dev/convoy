package newcloud

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

func (m *Migrator) RunSubscriptionMigration() error {
	for _, p := range m.projects {
		subscriptions, err := m.loadProjectSubscriptions(p.OrganisationID, p.UID, pagedResponse{})
		if err != nil {
			return err
		}

		err = m.SaveSubscriptions(context.Background(), subscriptions)
		if err != nil {
			return fmt.Errorf("failed to save subscriptions: %v", err)
		}
		return nil
	}

	return nil
}

const (
	createSubscription = `
    INSERT INTO convoy.subscriptions (
    id,name,type,
	project_id,endpoint_id,device_id,
	source_id,alert_config_count,alert_config_threshold,
	retry_config_type,retry_config_duration,
	retry_config_retry_count,filter_config_event_types,
	filter_config_filter_headers,filter_config_filter_body,
	rate_limit_config_count,rate_limit_config_duration,
	created_at, updated_at, deleted_at,function
	)
    VALUES (
        :id, :name, :type,
		:project_id, :endpoint_id, :device_id,
		:source_id, :alert_config_count, :alert_config_threshold,
		:retry_config_type, :retry_config_duration,
		:retry_config_retry_count, :filter_config_event_types,
		:filter_config_filter_headers, :filter_config_filter_body,
		:rate_limit_config_count, :rate_limit_config_duration,
		:created_at, :updated_at, :deleted_at, :function
	);
    `
)

func (s *Migrator) SaveSubscriptions(ctx context.Context, subscriptions []datastore.Subscription) error {
	values := make([]map[string]interface{}, 0, len(subscriptions))
	for _, subscription := range subscriptions {

		ac := subscription.GetAlertConfig()
		rc := subscription.GetRetryConfig()
		fc := subscription.GetFilterConfig()
		rlc := subscription.GetRateLimitConfig()

		var endpointID, sourceID, deviceID *string
		if !util.IsStringEmpty(subscription.EndpointID) {
			endpointID = &subscription.EndpointID
		}

		if !util.IsStringEmpty(subscription.SourceID) {
			sourceID = &subscription.SourceID
		}

		if !util.IsStringEmpty(subscription.DeviceID) {
			deviceID = &subscription.DeviceID
		}

		values = append(values, map[string]interface{}{
			"id":                           subscription.UID,
			"name":                         subscription.Name,
			"type":                         subscription.Type,
			"project_id":                   subscription.ProjectID,
			"endpoint_id":                  endpointID,
			"device_id":                    deviceID,
			"source_id":                    sourceID,
			"alert_config_count":           ac.Count,
			"alert_config_threshold":       ac.Threshold,
			"retry_config_type":            rc.Type,
			"retry_config_duration":        rc.Duration,
			"retry_config_retry_count":     rc.RetryCount,
			"filter_config_event_types":    fc.EventTypes,
			"filter_config_filter_headers": fc.Filter.Headers,
			"filter_config_filter_body":    fc.Filter.Body,
			"rate_limit_config_count":      rlc.Count,
			"rate_limit_config_duration":   rlc.Duration,
			"created_at":                   subscription.CreatedAt,
			"updated_at":                   subscription.UpdatedAt,
			"deleted_at":                   subscription.DeletedAt,
			"function":                     subscription.Function,
		})

	}

	_, err := s.newDB.NamedExecContext(ctx, createSubscription, values)
	return err
}
