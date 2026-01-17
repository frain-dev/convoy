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
			log.WithError(err).Error("Failed to create PubSubSource")
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
	defer handlePanic(source)

	fmt.Printf("[HANDLER] handler() called for source %s (project: %s)\n", source.UID, source.ProjectID)
	fmt.Printf("[HANDLER] Message body: %s\n", msg)
	i.log.Infof("AMQP handler called for source %s (project: %s), message: %s", source.UID, source.ProjectID, msg)

	// unmarshal to an interface{} struct
	fmt.Printf("[HANDLER] About to unmarshal message\n")
	var raw any
	if err := json.Unmarshal([]byte(msg), &raw); err != nil {
		fmt.Printf("[HANDLER] ERROR: Failed to unmarshal: %v\n", err)
		log.WithError(err).Error("Failed to unmarshal AMQP message")
		return err
	}
	fmt.Printf("[HANDLER] Message unmarshalled successfully\n")

	type ConvoyEvent struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		CustomHeaders  map[string]string `json:"custom_headers"`
		IdempotencyKey string            `json:"idempotency_key"`
	}

	fmt.Printf("[HANDLER] Checking for body transformation function\n")
	var payload any
	if source.BodyFunction != nil && !util.IsStringEmpty(*source.BodyFunction) {
		fmt.Printf("[HANDLER] Applying body transformation\n")
		t := transform.NewTransformer()
		p, _, err := t.Transform(*source.BodyFunction, raw)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Body transformation failed: %v\n", err)
			return err
		}

		payload = p
	} else {
		fmt.Printf("[HANDLER] No body transformation, using raw payload\n")
		payload = raw
	}

	// transform to required payload
	fmt.Printf("[HANDLER] Marshalling payload to bytes\n")
	pBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("[HANDLER] ERROR: Failed to marshal payload: %v\n", err)
		return err
	}

	fmt.Printf("[HANDLER] Creating decoder with DisallowUnknownFields\n")
	var convoyEvent ConvoyEvent
	decoder := json.NewDecoder(bytes.NewReader(pBytes))
	decoder.DisallowUnknownFields()

	// check the payload structure to be sure it satisfies what convoy can ingest else discard and nack it.
	fmt.Printf("[HANDLER] Decoding into ConvoyEvent struct\n")
	if err = decoder.Decode(&convoyEvent); err != nil {
		fmt.Printf("[HANDLER] ERROR: Decode failed: %v\n", err)
		log.WithError(err).Errorf("the payload for %s with id (%s) is badly formatted, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}
	fmt.Printf("[HANDLER] Decoded successfully: endpoint_id=%s, event_type=%s\n", convoyEvent.EndpointID, convoyEvent.EventType)

	fmt.Printf("[HANDLER] Validating event_type (value: %s)\n", convoyEvent.EventType)
	if util.IsStringEmpty(convoyEvent.EventType) {
		fmt.Printf("[HANDLER] ERROR: Empty event_type\n")
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include an event type, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}

	fmt.Printf("[HANDLER] Validating data (length: %d)\n", len(convoyEvent.Data))
	if len(convoyEvent.Data) == 0 {
		fmt.Printf("[HANDLER] ERROR: Empty data\n")
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include any data, please refer to the documentation or"+
			" use transform functions to properly format it, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		return err
	}
	fmt.Printf("[HANDLER] Validation passed\n")

	fmt.Printf("[HANDLER] Decoding message headers\n")
	headerMap := map[string]string{}
	err = msgpack.DecodeMsgPack(metadata, &headerMap)
	if err != nil {
		fmt.Printf("[HANDLER] ERROR: Failed to decode headers: %v\n", err)
		return err
	}
	fmt.Printf("[HANDLER] Headers decoded, count: %d\n", len(headerMap))

	mergeHeaders(headerMap, convoyEvent.CustomHeaders)
	fmt.Printf("[HANDLER] Headers merged, total count: %d\n", len(headerMap))

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
	fmt.Printf("[HANDLER] Message type from headers: '%s'\n", messageType)
	fmt.Printf("[HANDLER] Entering switch statement\n")
	switch messageType {
	case "single":
		fmt.Printf("[HANDLER] Matched 'single' case\n")
		id := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: id}.SingleJobId()
		fmt.Printf("[HANDLER] Created IDs: event_id=%s, job_id=%s\n", id, jobId)
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

		fmt.Printf("[HANDLER] Checking if endpoint_id is empty: %s\n", ce.Params.EndpointID)
		if util.IsStringEmpty(ce.Params.EndpointID) {
			fmt.Printf("[HANDLER] ERROR: Empty endpoint_id\n")
			return fmt.Errorf("the payload with message type %s for %s with id (%s) doesn't include an endpoint id, please refer to the documentation or"+
				" use transform functions to properly format it, got: %q (event_type: %q)", messageType, source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		}
		fmt.Printf("[HANDLER] Endpoint ID is valid, checking in database\n")

		// check if the endpoint_id is valid
		fmt.Printf("[HANDLER] Calling FindEndpointByID: endpoint=%s, project=%s\n", ce.Params.EndpointID, ce.Params.ProjectID)
		_, err = i.endpointRepo.FindEndpointByID(ctx, ce.Params.EndpointID, ce.Params.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				fmt.Printf("[HANDLER] ERROR: Endpoint not found\n")
				return fmt.Errorf("the payload for %s with id (%s) includes an invalid endpoint id, got: %q (event_type: %q)", source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
			}
			fmt.Printf("[HANDLER] ERROR: Database error finding endpoint: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Endpoint found successfully\n")

		fmt.Printf("[HANDLER] Encoding CreateEvent to msgpack\n")
		eventByte, err := msgpack.EncodeMsgPack(ce)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to encode: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Encoded successfully, bytes length: %d\n", len(eventByte))

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}
		fmt.Printf("[HANDLER] Created job struct\n")

		// write to our queue if it's a normal event
		fmt.Printf("[HANDLER] Writing CreateEvent job to queue: processor=%s, queue=%s\n", convoy.CreateEventProcessor, convoy.CreateEventQueue)
		i.log.Infof("Writing CreateEvent job to queue for event %s (endpoint: %s, project: %s)", id, ce.Params.EndpointID, ce.Params.ProjectID)
		err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to write to queue: %v\n", err)
			log.WithError(err).Error("Failed to write CreateEvent job to queue")
			return err
		}
		fmt.Printf("[HANDLER] Successfully wrote to queue\n")
		i.log.Infof("Successfully wrote CreateEvent job to queue for event %s", id)
	case "fanout":
		fmt.Printf("[HANDLER] Matched 'fanout' case\n")
		id := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: id}.FanOutJobId()
		fmt.Printf("[HANDLER] Created IDs: event_id=%s, job_id=%s\n", id, jobId)
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

		fmt.Printf("[HANDLER] Checking owner_id: %s\n", ce.Params.OwnerID)
		if util.IsStringEmpty(ce.Params.OwnerID) {
			fmt.Printf("[HANDLER] ERROR: Empty owner_id\n")
			return fmt.Errorf("the payload with message type %s for %s with id (%s) doesn't include an owner id, please refer to the documentation or"+
				" use transform functions to properly format it, got: %q (event_type: %q)", messageType, source.Name, source.UID, convoyEvent.EndpointID, convoyEvent.EventType)
		}

		fmt.Printf("[HANDLER] Encoding fanout CreateEvent to msgpack\n")
		eventByte, err := msgpack.EncodeMsgPack(ce)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to encode: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Encoded successfully, bytes length: %d\n", len(eventByte))

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}

		// write to our queue if it's a normal event
		fmt.Printf("[HANDLER] Writing fanout CreateEvent job to queue: processor=%s, queue=%s\n", convoy.CreateEventProcessor, convoy.CreateEventQueue)
		i.log.Infof("Writing fanout CreateEvent job to queue for event %s (owner: %s, project: %s)", id, ce.Params.OwnerID, ce.Params.ProjectID)
		err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to write to queue: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Successfully wrote fanout job to queue\n")
		i.log.Infof("Successfully wrote fanout CreateEvent job to queue for event %s", id)
	case "broadcast":
		fmt.Printf("[HANDLER] Matched 'broadcast' case\n")
		eventId := ulid.Make().String()
		jobId := queue.JobId{ProjectID: source.ProjectID, ResourceID: eventId}.BroadcastJobId()
		fmt.Printf("[HANDLER] Created IDs: event_id=%s, job_id=%s\n", eventId, jobId)
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

		fmt.Printf("[HANDLER] Encoding broadcast event to msgpack\n")
		eventByte, err := msgpack.EncodeMsgPack(broadcastEvent)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to encode: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Encoded successfully, bytes length: %d\n", len(eventByte))

		job := &queue.Job{
			ID:      jobId,
			Payload: eventByte,
		}

		// write to our queue if it's a broadcast event
		fmt.Printf("[HANDLER] Writing broadcast event job to queue: processor=%s, queue=%s\n", convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue)
		i.log.Infof("Writing broadcast event job to queue for event %s (project: %s)", eventId, broadcastEvent.ProjectID)
		err = i.queue.Write(convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			fmt.Printf("[HANDLER] ERROR: Failed to write to queue: %v\n", err)
			return err
		}
		fmt.Printf("[HANDLER] Successfully wrote broadcast job to queue\n")
		i.log.Infof("Successfully wrote broadcast event job to queue for event %s", eventId)
	default:
		err = fmt.Errorf("%s isn't a valid pubsub message type, it should be one of single, fanout or broadcast", messageType)
		log.Error(err)
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

func handlePanic(source *datastore.Source) {
	if err := recover(); err != nil {
		fmt.Printf("[PANIC] Recovered from panic in source %s (id: %s): %v\n", source.Name, source.UID, err)
		log.Error(fmt.Errorf("recovered from panic, source %s with id: %s crashed with error: %s", source.Name, source.UID, err))
	}
}
