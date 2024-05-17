package listener

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/r3labs/diff/v3"
)

type ProjectListener struct {
	queue queue.Queuer
}

func NewProjectListener(queue queue.Queuer) *ProjectListener {
	return &ProjectListener{queue: queue}
}

func (e *ProjectListener) AfterUpdate(data interface{}, changelog interface{}) {
	e.run(string(datastore.ProjectUpdated), data, changelog)
}

func (e *ProjectListener) run(eventType string, data interface{}, changelog interface{}) {
	project, ok := data.(*datastore.Project)
	if !ok {
		log.Errorf("invalid type for project - %s", eventType)
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
							log.WithError(err).Errorf("%s is not a valid time duration", project.Config.SearchPolicy)
							return
						}

						params := datastore.SearchIndexParams{
							ProjectID: project.UID,
							Interval:  int(dur.Hours()),
						}

						bytes, err := json.Marshal(params)
						if err != nil {
							log.WithError(err).Error("an error occurred marshalling the payload")
							return
						}

						job := &queue.Job{
							ID:      project.UID,
							Payload: bytes,
							Delay:   1 * time.Second,
						}

						err = e.queue.Write(convoy.TokenizeSearchForProject, convoy.ScheduleQueue, job)
						if err != nil {
							log.WithError(err).Error("an error occurred writing the job to the queue")
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
