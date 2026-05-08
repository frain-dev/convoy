package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/r3labs/diff/v3"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type ProjectListener struct {
	queue  queue.Queuer
	logger log.Logger
}

func NewProjectListener(queue queue.Queuer, logger log.Logger) *ProjectListener {
	return &ProjectListener{queue: queue, logger: logger}
}

func (e *ProjectListener) AfterUpdate(ctx context.Context, data, changelog interface{}) {
	e.run(ctx, datastore.ProjectUpdated, data, changelog)
}

func (e *ProjectListener) run(ctx context.Context, eventType datastore.HookEventType, data, changelog interface{}) {
	project, ok := data.(*datastore.Project)
	if !ok {
		e.logger.Error(fmt.Sprintf("invalid type for project - %s", eventType))
		return
	}

	if changelog != nil {
		if clog, okk := changelog.(diff.Changelog); okk {
			for _, change := range clog {
				switch change.Type {
				case diff.UPDATE:
					if testSliceEq(change.Path, []string{"Config", "RetentionPolicy", "SearchPolicy"}) {
						dur, err := time.ParseDuration(project.Config.SearchPolicy)
						if err != nil {
							e.logger.Error(fmt.Sprintf("%s is not a valid time duration: %v", project.Config.SearchPolicy, err))
							return
						}

						params := datastore.SearchIndexParams{
							ProjectID: project.UID,
							Interval:  int(dur.Hours()),
						}

						bytes, err := json.Marshal(params)
						if err != nil {
							e.logger.Error("an error occurred marshalling the payload", "error", err)
							return
						}

						job := &queue.Job{
							ID:      project.UID,
							Payload: bytes,
							Delay:   1 * time.Second,
						}

						err = e.queue.Write(ctx, convoy.TokenizeSearchForProject, convoy.ScheduleQueue, job)
						if err != nil {
							e.logger.Error("an error occurred writing the job to the queue", "error", err)
							return
						}
					}
				default:
					continue
				}
			}
		}
	}
}

func testSliceEq(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
