package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
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

	metaData := &datastore.Metadata{
		NumTrials:       0,
		RetryLimit:      project.Config.Strategy.RetryCount,
		Data:            mpByte,
		Raw:             string(mpByte),
		IntervalSeconds: project.Config.Strategy.Duration,
		Strategy:        project.Config.Strategy.Type,
		NextSendTime:    time.Now(),
	}

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: projectID,
		EventType: eventType,
		Status:    datastore.ScheduledEventStatus,
		Metadata:  metaData,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = m.metaEventRepo.CreateMetaEvent(context.Background(), metaEvent)
	if err != nil {
		log.WithError(err).Error("failed to create meta event")
		return err
	}

	s := task.MetaEvent{
		MetaEventID: metaEvent.UID,
		ProjectID:   projectID,
	}

	mE, err := json.Marshal(s)
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
