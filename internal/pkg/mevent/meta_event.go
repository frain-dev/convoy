package mevent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/oklog/ulid/v2"
)

type MetaEvent struct {
	queue         queue.Queuer
	projectRepo   datastore.ProjectRepository
	metaEventRepo datastore.MetaEventRepository
}

func NewMetaEvent(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) *MetaEvent {
	return &MetaEvent{queue: queue, projectRepo: projectRepo, metaEventRepo: metaEventRepo}
}

func (m *MetaEvent) Run(eventType string, projectID string, data interface{}) error {
	project, err := m.projectRepo.FetchProjectByID(context.Background(), projectID)
	if err != nil {
		return err
	}

	cfg := project.Config.MetaEvent
	if !cfg.IsEnabled {
		return nil
	}

	if !m.isSubscribed(eventType, cfg.EventType) {
		return nil
	}

	dByte, err := json.Marshal(data)
	if err != nil {
		return err
	}

	mP := datastore.MetaEventPayload{
		EventType: eventType,
		Data:      dByte,
	}

	mpByte, err := json.Marshal(mP)
	if err != nil {
		return err
	}

	metaEvent := &datastore.MetaEvent{
		UID:           ulid.Make().String(),
		ProjectID:     projectID,
		EventType:     eventType,
		Status:        string(datastore.ScheduledEventStatus),
		RetryCount:    1,
		MaxRetryCount: 3,
		Data:          mpByte,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = m.metaEventRepo.CreateMetaEvent(context.Background(), metaEvent)
	if err != nil {
		log.WithError(err).Error("failed to create meta event")
		return err
	}

	mE, err := json.Marshal(metaEvent)
	if err != nil {
		return err
	}

	err = m.queue.Write(convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{
		ID:      metaEvent.UID,
		Payload: mE,
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *MetaEvent) isSubscribed(eventType string, events []string) bool {
	for _, event := range events {
		if event == eventType {
			return true
		}
	}

	return false
}
