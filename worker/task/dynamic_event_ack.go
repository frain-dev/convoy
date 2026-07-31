package task

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func publishDynamicEventAck(ctx context.Context, rdb redis.UniversalClient, logger log.Logger, projectID, eventID string, result dynamiceventack.Result) {
	if rdb == nil {
		return
	}
	if err := dynamiceventack.Publish(ctx, rdb, projectID, eventID, result); err != nil && logger != nil {
		logger.ErrorContext(ctx, "failed to publish dynamic event sync ack", "project_id", projectID, "event_id", eventID, "error", err)
	}
}
