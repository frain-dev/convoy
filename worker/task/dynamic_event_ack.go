package task

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func publishDynamicEventAck(ctx context.Context, acker dynamiceventack.Acker, logger log.Logger, projectID, eventID string, result dynamiceventack.Result) {
	if acker == nil {
		return
	}
	if err := acker.Publish(ctx, projectID, eventID, result); err != nil && logger != nil {
		logger.ErrorContext(ctx, "failed to publish dynamic event sync ack", "project_id", projectID, "event_id", eventID, "error", err)
	}
}
