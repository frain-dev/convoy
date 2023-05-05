package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/hibiken/asynq"
)

var (
	ErrProjectIdFieldIsRequired  = errors.New("project_id field should be a string")
	ErrProjectIdFieldIsNotString = errors.New("project_id field does not exist on the document")
)

type IndexDocument struct {
	searchBackend searcher.Searcher
}

func NewIndexDocument(cfg config.Configuration) (*IndexDocument, error) {
	search, err := searcher.NewSearchClient(cfg)
	if err != nil {
		return nil, err
	}

	return &IndexDocument{
		searchBackend: search,
	}, nil
}

func (id *IndexDocument) ProcessTask(ctx context.Context, t *asynq.Task) error {
	buf := t.Payload()

	var event map[string]interface{}
	err := json.Unmarshal(buf, &event)
	if err != nil {
		return &EndpointError{Err: err, delay: defaultDelay}
	}

	event["id"] = event["uid"]
	if g, found := event["project_id"]; found {
		if project_id, ok := g.(string); ok {
			err = id.searchBackend.Index(project_id, event)
			if err != nil {
				return &EndpointError{Err: err, delay: time.Second * 5}
			}
		} else {
			return &EndpointError{Err: ErrProjectIdFieldIsNotString, delay: time.Second * 1}
		}
	} else {
		return &EndpointError{Err: ErrProjectIdFieldIsRequired, delay: time.Second * 1}
	}

	return nil
}
