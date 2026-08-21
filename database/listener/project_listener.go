package listener

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type ProjectListener struct {
	logger log.Logger
}

func NewProjectListener(_ queue.Queuer, logger log.Logger) *ProjectListener {
	return &ProjectListener{logger: logger}
}

func (e *ProjectListener) AfterUpdate(_ context.Context, data, _ interface{}) {
	// Project updates no longer enqueue TokenizeSearchForProject; payload search
	// uses the active date filter and a license gate instead of search_policy.
	if _, ok := data.(*datastore.Project); !ok {
		e.logger.Error("invalid type for project update")
	}
}
