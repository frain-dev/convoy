package services

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

type MetaEventService struct {
	Queue         queue.Queuer
	MetaEventRepo datastore.MetaEventRepository
}

func (m *MetaEventService) Run(ctx context.Context, metaEvent *datastore.MetaEvent) error {
	metaEvent.Status = datastore.ScheduledEventStatus
	err := m.MetaEventRepo.UpdateMetaEvent(ctx, metaEvent.ProjectID, metaEvent)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update meta event")
		return err
	}

	payload := task.MetaEvent{
		MetaEventID: metaEvent.UID,
		ProjectID:   metaEvent.ProjectID,
	}

	bytes, err := msgpack.EncodeMsgPack(payload)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to marshal meta event payload")
		return err
	}

	err = m.Queue.Write(convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{
		ID:      metaEvent.UID,
		Payload: bytes,
	})
	if err != nil {
		return fmt.Errorf("error occurred re-enqueing meta event - %s: %v", metaEvent.UID, err)
	}

	return nil
}
