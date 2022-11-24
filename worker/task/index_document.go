package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
)

var ErrGroupIdFieldIsRequired = errors.New("group_id field should be a string")
var ErrGroupIdFieldIsNotString = errors.New("group_id field does not exist on the document")

func SearchIndex(search searcher.Searcher) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		if search == nil {
			return nil
		}

		buf := t.Payload()

		var event datastore.Event
		err := json.Unmarshal(buf, &event)
		if err != nil {
			log.WithError(err).Error("[json]: failed to unmarshal event payload")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		// convert event data field to map
		rawData := event.Data
		var eventData *convoy.GenericMap
		err = json.Unmarshal(rawData, &eventData)
		if err != nil {
			return err
		}

		// convert event to bytes
		eBytes, err := json.Marshal(event)
		if err != nil {
			return err
		}

		// convert event to map
		var document convoy.GenericMap
		err = json.Unmarshal(eBytes, &document)
		if err != nil {
			return err
		}

		document["data"] = eventData
		document["id"] = document["uid"]

		if g, found := document["group_id"]; found {
			if group_id, ok := g.(string); ok {
				err = search.Index(group_id, document)
				if err != nil {
					log.Errorf("[typesense] error indexing event: %s", err)
					return &EndpointError{Err: err, delay: time.Second * 5}
				}
			} else {
				log.Errorf("[typesense] error indexing event: %s", ErrGroupIdFieldIsNotString)
				return &EndpointError{Err: ErrGroupIdFieldIsNotString, delay: time.Second * 1}
			}
		} else {
			log.Errorf("[typesense] error indexing event: %s", ErrGroupIdFieldIsRequired)
			return &EndpointError{Err: ErrGroupIdFieldIsRequired, delay: time.Second * 1}
		}

		return nil
	}
}
