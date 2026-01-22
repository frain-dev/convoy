package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	common "github.com/frain-dev/convoy/internal/pkg/pubsub/const"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/transform"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
)

type IngestCtxKey string

var ingestCtx IngestCtxKey = "IngestCtx"

type Ingest struct {
	ctx          context.Context
	ticker       *time.Ticker
	queue        queue.Queuer
	rateLimiter  limiter.RateLimiter
	sources      map[memorystore.Key]*PubSubSource
	table        *memorystore.Table
	log          log.StdLogger
	instanceId   string
	licenser     license.Licenser
	endpointRepo datastore.EndpointRepository
}

func NewIngest(ctx context.Context, table *memorystore.Table, queue queue.Queuer, log log.StdLogger,
	rateLimiter limiter.RateLimiter, licenser license.Licenser, instanceId string, endpointRepo datastore.EndpointRepository) (*Ingest, error) {
	ctx = context.WithValue(ctx, ingestCtx, nil)
	i := &Ingest{
		ctx:          ctx,
		log:          log,
		table:        table,
		queue:        queue,
		rateLimiter:  rateLimiter,
		instanceId:   instanceId,
		licenser:     licenser,
		sources:      make(map[memorystore.Key]*PubSubSource),
		ticker:       time.NewTicker(time.Duration(1) * time.Second),
		endpointRepo: endpointRepo,
	}

	return i, nil
}

// Run is the core of the ingester. It does the following in an infinite loop:
// 1. Loop through the sources at intervals
// 2. Compare the retrieved sources with the running
// 3. Cancels deleted sources.
// 4. Starts new sources.
func (i *Ingest) Run() {
	i.log.Infof("Ingest.Run() started - waiting for ticker")
	for {
		select {
		// retrieve new sources
		case <-i.ticker.C:
			i.log.Infof("Ingest ticker fired")
			err := i.run()
			if err != nil {
				i.log.WithError(err).Error("ingest runner failed")
			}

		case <-i.ctx.Done():
			i.log.Infof("Ingest context cancelled - stopping")
			// stop ticker.
			i.ticker.Stop()

			// clean up. :)
			return
		}
	}
}

func (i *Ingest) getSourceKeys() []memorystore.Key {
	s := make([]memorystore.Key, 0, len(i.sources))
	for k := range i.sources {
		s = append(s, k)
	}

	return s
}

func (i *Ingest) run() error {
	i.log.Infof("Ingest.run() called - checking for source changes")
	i.log.Infof("Current sources in table: %d, running sources: %d", len(i.table.GetKeys()), len(i.sources))

	// cancel all stale/outdated source runners.
	staleRows := memorystore.Difference(i.getSourceKeys(), i.table.GetKeys())
	i.log.Infof("Stale sources to remove: %d", len(staleRows))
	for _, key := range staleRows {
		ps, ok := i.sources[key]
		if !ok {
			continue
		}

		ps.Stop()
		delete(i.sources, key)
	}

	// start all new/updated source runners.
	newSourceKeys := memorystore.Difference(i.table.GetKeys(), i.getSourceKeys())
	i.log.Infof("New sources to start: %d", len(newSourceKeys))
	for _, key := range newSourceKeys {
		sr := i.table.Get(key)
		if sr == nil {
			continue
		}

		ss, ok := sr.Value().(datastore.Source)
		if !ok {
			return errors.New("invalid source in memory store")
		}

		i.log.Infof("Starting new source: %s (type: %s, project: %s)", ss.UID, ss.Type, ss.ProjectID)
		ps, err := NewPubSubSource(i.ctx, &ss, i.handler, i.log, i.rateLimiter, i.licenser, i.instanceId)
		if err != nil {
			i.log.WithError(err).Error("Failed to create PubSubSource")
			return err
		}

		// ps.hash = key
		ps.Start()
		i.log.Infof("Started source: %s", ss.UID)
		i.sources[key] = ps
	}

	return nil
}

func (i *Ingest) handler(ctx context.Context, source *datastore.Source, msg string, metadata []byte) error {
	defer i.handlePanic(source)

	i.log.Infof("AMQP handler called for source %s (project: %s), message: %s", source.UID, source.ProjectID, msg)

	// unmarshal to an interface{} struct
	var raw any
	if err := json.Unmarshal([]byte(msg), &raw); err != nil {
		i.log.WithError(err).Error("Failed to unmarshal AMQP message")
		return err
	}

	type ConvoyEvent struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		CustomHeaders  map[string]string `json:"custom_headers"`
		IdempotencyKey string            `json:"idempotency_key"`
	}

	var payload any
	if source.BodyFunction != nil && !util.IsStringEmpty(*source.BodyFunction) {
		t := transform.NewTransformer()
		p, _, err := t.Transform(*source.BodyFunction, raw)
		if err != nil {
			return err
		}

		payload = p
	} else {
		payload = raw
	}

	// transform to required payload
	pBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var convoyEvent ConvoyEvent
	decoder := json.NewDecoder(bytes.NewReader(pBytes))
	decoder.DisallowUnknownFields()

	// check the payload structure to be sure it satisfies what convoy can ingest else discard and nack it.
	if err = decoder.Decode(&convoyEvent); err != nil {
		i.log.WithError(err).Errorf("the payload for %s with id (%s) is badly formatted, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}

	if util.IsStringEmpty(convoyEvent.EventType) {
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include an event type, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}

	if len(convoyEvent.Data) == 0 {
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include any data, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}

	headerMap := map[string]string{}
	err = msgpack.DecodeMsgPack(metadata, &headerMap)
	if err != nil {
		return err
	}

	mergeHeaders(headerMap, convoyEvent.CustomHeaders)

	headers := map[string]string{}
	if source.HeaderFunction != nil && !util.IsStringEmpty(*source.HeaderFunction) {
		t := transform.NewTransformer()
		h, _, transErr := t.Transform(*source.HeaderFunction, headerMap)
		if transErr != nil {
			return transErr
		}

		switch castedH := h.(type) {
		case map[string]any:
			for k, v := range castedH {
				if _, ok := headers[k]; ok {
					continue
				}

				if _, ok := v.(string); !ok {
					return fmt.Errorf("headers values for %s with id (%s) should be strings, want: type of string, got: %+v, of type %T", source.Name, source.UID, v, v)
				}

				headers[k] = v.(string)
			}
		case map[string]string:
			headers = castedH
		default:
			return fmt.Errorf("the headers for %s with id (%s) are badly formatted, want: type of map[string]any or map[string]string, got: %+v, of type %T", source.Name, source.UID, castedH, castedH)
		}
	} else {
		headers = headerMap
	}

	messageType := headers[common.ConvoyMessageTypeHeader]
	switch messageType {
	case "single":
		id := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: id}.SingleJobId()
		ce := task.CreateEvent{
			JobID: jobId,
			Params: task.CreateEventTaskParams{
				UID:            id,
				SourceID:       source.UID,
				ProjectID:      source.ProjectID,
				EndpointID:     convoyEvent.EndpointID,
				EventType:      convoyEvent.EventType,
				Data:           convoyEvent.Data,
				CustomHeaders:  headers,
				IdempotencyKey: convoyEvent.IdempotencyKey,
				AcknowledgedAt: time.Now(),
			},
			CreateSubscription: !util.IsStringEmpty(convoyEvent.EndpointID),
		}

		if util.IsStringEmpty(ce.Params.EndpointID) {
			return fmt.Errorf("the payload with message type %s for %s with id (%s) doesn't include an endpoint id, please refer to the documentation or"+
				" use transform functions to properly format it, got: %q (event_type: %q)", messageType, source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		}

		// check if the endpoint_id is valid
		_, err = i.endpointRepo.FindEndpointByID(ctx, ce.Params.EndpointID, ce.Params.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				return fmt.Errorf("the payload for %s with id (%s) includes an invalid endpoint id, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
			}
			return err
		}

		eventByte, err := msgpack.EncodeMsgPack(ce)
		if err != nil {
			return err
		}

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}

		// write to our queue if it's a normal event
		i.log.Infof("Writing CreateEvent job to queue for event %s (endpoint: %s, project: %s)", id, ce.Params.EndpointID, ce.Params.ProjectID)
		err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			i.log.WithError(err).Error("Failed to write CreateEvent job to queue")
			return err
		}
		i.log.Infof("Successfully wrote CreateEvent job to queue for event %s", id)
	case "fanout":
		id := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: id}.FanOutJobId()
		ce := task.CreateEvent{
			JobID: jobId,
			Params: task.CreateEventTaskParams{
				UID:            id,
				CustomHeaders:  headers,
				SourceID:       source.UID,
				Data:           convoyEvent.Data,
				ProjectID:      source.ProjectID,
				OwnerID:        convoyEvent.OwnerID,
				EventType:      convoyEvent.EventType,
				EndpointID:     convoyEvent.EndpointID,
				IdempotencyKey: convoyEvent.IdempotencyKey,
				AcknowledgedAt: time.Now(),
			},
			CreateSubscription: !util.IsStringEmpty(convoyEvent.EndpointID),
		}

		if util.IsStringEmpty(ce.Params.OwnerID) {
			return fmt.Errorf("the payload with message type %s for %s with id (%s) doesn't include an owner id, please refer to the documentation or"+
				" use transform functions to properly format it, got: %q (event_type: %q)", messageType, source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		}

		eventByte, err := msgpack.EncodeMsgPack(ce)
		if err != nil {
			return err
		}

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}

		// write to our queue if it's a normal event
		i.log.Infof("Writing fanout CreateEvent job to queue for event %s (owner: %s, project: %s)", id, ce.Params.OwnerID, ce.Params.ProjectID)
		err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			return err
		}
		i.log.Infof("Successfully wrote fanout CreateEvent job to queue for event %s", id)
	case "broadcast":
		eventId := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: eventId}.BroadcastJobId()
		broadcastEvent := models.BroadcastEvent{
			JobID:          jobId,
			EventID:        eventId,
			ProjectID:      source.ProjectID,
			SourceID:       source.UID,
			EventType:      convoyEvent.EventType,
			Data:           convoyEvent.Data,
			CustomHeaders:  headers,
			IdempotencyKey: convoyEvent.IdempotencyKey,
			AcknowledgedAt: time.Now(),
		}

		eventByte, err := msgpack.EncodeMsgPack(broadcastEvent)
		if err != nil {
			return err
		}

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}

		// write to our queue if it's a broadcast event
		i.log.Infof("Writing broadcast event job to queue for event %s (project: %s)", eventId, broadcastEvent.ProjectID)
		err = i.queue.Write(convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			return err
		}
		i.log.Infof("Successfully wrote broadcast event job to queue for event %s", eventId)
	default:
		err = fmt.Errorf("%s isn't a valid pubsub message type, it should be one of single, fanout or broadcast", messageType)
		i.log.Error(err)
		return err
	}

	return nil
}

func mergeHeaders(dest, src map[string]string) {
	var k, v string
	// convert all the dest header values to lowercase
	for k, v = range dest {
		dest[strings.ToLower(k)] = v
	}

	// convert all the src header values to lowercase
	for k, v = range src {
		src[strings.ToLower(k)] = v
	}

	for k, v = range src {
		if _, ok := dest[k]; ok {
			continue
		}

		dest[k] = v
	}

	_, ok := dest[common.ConvoyMessageTypeHeader]
	if !ok {
		// the message type header wasn't found, set it to a default value
		dest[common.ConvoyMessageTypeHeader] = "single"
	}
}

func (i *Ingest) handlePanic(source *datastore.Source) {
	if err := recover(); err != nil {
		i.log.Error(fmt.Errorf("recovered from panic, source %s with id: %s crashed with error: %s", source.Name, source.UID, err))
	}
}
