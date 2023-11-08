package newcloud

import (
	"context"
	"fmt"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/database/postgres"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

func (m *Migrator) RunEventDeliveriesMigration() error {
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(m, ncache.NewNoopCache())
	pageable := datastore.Pageable{
		PerPage:    1000,
		Direction:  "next",
		NextCursor: "",
	}

	for _, p := range m.projects {
		events, err := m.loadEventDeliveries(eventDeliveryRepo, &p, pageable)
		if err != nil {
			return err
		}

		err = m.SaveEventDeliveries(context.Background(), events)
		if err != nil {
			return fmt.Errorf("failed to save events: %v", err)
		}
		return nil
	}

	return nil
}

const (
	saveEventDeliveries = `
    INSERT INTO convoy.event_deliveries (
          id, project_id, event_id, endpoint_id, device_id, subscription_id,
          headers, attempts, status, metadata, cli_metadata, description,
          created_at, updated_at, deleted_at
          )
    VALUES (
        :id, :project_id, :event_id, :endpoint_id, :device_id,
        :subscription_id, :headers, :attempts, :status, :metadata,
        :cli_metadata, :description, :created_at, :updated_at, :deleted_at
    )
    `
)

func (e *Migrator) SaveEventDeliveries(ctx context.Context, deliveries []datastore.EventDelivery) error {
	values := make([]map[string]interface{}, 0, len(deliveries))

	for _, delivery := range deliveries {
		var endpointID *string
		var deviceID *string

		if !util.IsStringEmpty(delivery.EndpointID) {
			endpointID = &delivery.EndpointID
		}

		if !util.IsStringEmpty(delivery.DeviceID) {
			deviceID = &delivery.DeviceID
		}

		values = append(values, map[string]interface{}{
			"id":              delivery.UID,
			"project_id":      delivery.ProjectID,
			"event_id":        delivery.EventID,
			"endpoint_id":     endpointID,
			"device_id":       deviceID,
			"subscription_id": delivery.SubscriptionID,
			"headers":         delivery.Headers,
			"attempts":        delivery.DeliveryAttempts,
			"status":          delivery.Status,
			"metadata":        delivery.Metadata,
			"cli_metadata":    delivery.CLIMetadata,
			"description":     delivery.Description,
			"created_at":      delivery.CreatedAt,
			"updated_at":      delivery.UpdatedAt,
			"deleted_at":      delivery.DeletedAt,
		})
	}

	_, err := e.newDB.NamedExecContext(ctx, saveEventDeliveries, values)
	return err
}
