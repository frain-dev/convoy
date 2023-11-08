package newcloud

import (
	"context"
	"database/sql"
	"fmt"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/database/postgres"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

func (m *Migrator) RunEventMigration() error {
	eventRepo := postgres.NewEventRepo(m, ncache.NewNoopCache())
	pageable := datastore.Pageable{
		PerPage:    1000,
		Direction:  "next",
		NextCursor: "",
	}

	for _, p := range m.projects {
		events, err := m.loadEvents(eventRepo, &p, pageable)
		if err != nil {
			return err
		}

		err = m.SaveEvents(context.Background(), events)
		if err != nil {
			return fmt.Errorf("failed to save events: %v", err)
		}
		return nil
	}

	return nil
}

const (
	saveEvents = `
	INSERT INTO convoy.events (
	id, event_type, endpoints, project_id, source_id, headers,
	raw, data,created_at,updated_at, deleted_at, url_query_params,
    idempotency_key, is_duplicate_event
    )
	VALUES (
	    :id, :event_type, :endpoints, :project_id, :source_id,
	    :headers, :raw, :data, :created_at, :updated_at, :deleted_at, :url_query_params,
        :idempotency_key,:is_duplicate_event
	)
	`

	createEventEndpoints = `
	INSERT INTO convoy.events_endpoints (endpoint_id, event_id) VALUES (:endpoint_id, :event_id)
	`
)

func (e *Migrator) SaveEvents(ctx context.Context, events []datastore.Event) error {
	ev := make([]map[string]interface{}, 0, len(events))
	evEndpoints := make([]postgres.EventEndpoint, 0, len(events)*2)

	for _, event := range events {
		var sourceID *string

		if !util.IsStringEmpty(event.SourceID) {
			sourceID = &event.SourceID
		}

		ev = append(ev, map[string]interface{}{
			"id":                 event.UID,
			"event_type":         event.EventType,
			"endpoints":          event.Endpoints,
			"project_id":         event.ProjectID,
			"source_id":          sourceID,
			"headers":            event.Headers,
			"raw":                event.Raw,
			"data":               event.Data,
			"created_at":         event.CreatedAt,
			"updated_at":         event.UpdatedAt,
			"deleted_at":         event.DeletedAt,
			"url_query_params":   event.URLQueryParams,
			"idempotency_key":    event.IdempotencyKey,
			"is_duplicate_event": event.IsDuplicateEvent,
		})

		if len(event.Endpoints) > 0 {
			for _, endpointID := range event.Endpoints {
				evEndpoints = append(evEndpoints, postgres.EventEndpoint{EventID: event.UID, EndpointID: endpointID})
			}
		}
	}

	tx, err := e.newDB.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	defer rollbackTx(tx)

	_, err = tx.NamedExecContext(ctx, saveEvents, ev)
	if err != nil {
		return err
	}

	if len(evEndpoints) > 0 {
		_, err = tx.NamedExecContext(ctx, createEventEndpoints, evEndpoints)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
