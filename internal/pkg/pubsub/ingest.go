package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/pkg/transform"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"
)

type IngestCtxKey string

var ingestCtx IngestCtxKey = "IngestCtx"

const ConvoyMessageTypeHeader = "x-convoy-message-type"

type Ingest struct {
	ctx     context.Context
	ticker  *time.Ticker
	queue   queue.Queuer
	sources map[string]*PubSubSource
	table   *memorystore.Table
	log     log.StdLogger
}

func NewIngest(ctx context.Context, table *memorystore.Table, queue queue.Queuer, log log.StdLogger) (*Ingest, error) {
	ctx = context.WithValue(ctx, ingestCtx, nil)
	i := &Ingest{
		ctx:     ctx,
		queue:   queue,
		log:     log,
		table:   table,
		sources: make(map[string]*PubSubSource),
		ticker:  time.NewTicker(time.Duration(1) * time.Second),
	}

	return i, nil
}

// Run is the core of the ingester. It does the following in an infinite loop:
// 1. Loop through the sources at intervals
// 2. Compare the retrieved sources with the running
// 3. Cancels deleted sources.
// 4. Starts new sources.
func (i *Ingest) Run() {
	for {
		select {
		// retrieve new sources
		case <-i.ticker.C:
			err := i.run()
			if err != nil {
				i.log.WithError(err).Error("ingest runner failed")
			}

		case <-i.ctx.Done():
			// stop ticker.
			i.ticker.Stop()

			// clean up. :)
		}
	}
}

func (i *Ingest) getSourceKeys() []string {
	var s []string
	for k := range i.sources {
		s = append(s, k)
	}

	return s
}

func (i *Ingest) run() error {
	i.log.Info("refreshing runner...", len(i.sources))

	// cancel all stale/outdated source runners.
	staleRows := util.Difference(i.getSourceKeys(), i.table.GetKeys())
	for _, key := range staleRows {
		ps, ok := i.sources[key]
		if !ok {
			continue
		}

		ps.Stop()
		delete(i.sources, key)
	}

	// start all new/updated source runners.
	newSourceKeys := util.Difference(i.table.GetKeys(), i.getSourceKeys())
	for _, key := range newSourceKeys {
		sr := i.table.Get(key)
		if sr == nil {
			continue
		}

		ss, ok := sr.Value().(datastore.Source)
		if !ok {
			return errors.New("invalid source in memory store")
		}

		ps, err := NewPubSubSource(i.ctx, &ss, i.handler, i.log)
		if err != nil {
			return err
		}

		ps.hash = key
		ps.Start()
		i.sources[key] = ps
	}

	return nil
}

func (i *Ingest) handler(_ context.Context, source *datastore.Source, msg string, metadata []byte) error {
	defer handlePanic(source)

	// unmarshal to an interface{} struct
	var raw any
	if err := json.Unmarshal([]byte(msg), &raw); err != nil {
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
	if source.BodyFunction.Ptr() != nil && !util.IsStringEmpty(source.BodyFunction.String) {
		t := transform.NewTransformer()
		p, _, err := t.Transform(source.BodyFunction.String, raw)
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
		log.WithError(err).Errorf("the payload for %s with id (%s) is badly formatted, please refer to the documentation or"+
			" use transfrom functions to properly format it, got: %+v", source.Name, source.UID, payload)
		return err
	}

	if util.IsStringEmpty(convoyEvent.EventType) {
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include an event type, please refer to the documentation or"+
			" use transfrom functions to properly format it, got: %+v", source.Name, source.UID, convoyEvent)
		return err
	}

	if len(convoyEvent.Data) == 0 {
		err := fmt.Errorf("the payload for %s with id (%s) doesn't include any data, please refer to the documentation or"+
			" use transfrom functions to properly format it, got: %+v", source.Name, source.UID, convoyEvent)
		return err
	}

	headerMap := map[string]string{}
	err = msgpack.DecodeMsgPack(metadata, &headerMap)
	if err != nil {
		return err
	}

	mergeHeaders(headerMap, convoyEvent.CustomHeaders)

	headers := map[string]string{}
	if source.HeaderFunction.Ptr() != nil && !util.IsStringEmpty(source.HeaderFunction.String) {
		t := transform.NewTransformer()
		h, _, transErr := t.Transform(source.HeaderFunction.String, headerMap)
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

	//fmt.Printf("payload: %s\n %+v\n %+v\n\n", source.Name, payload, headers)

	messageType := headers[ConvoyMessageTypeHeader]
	switch messageType {
	case "single":
		ce := task.CreateEvent{
			Params: task.CreateEventTaskParams{
				UID:            ulid.Make().String(),
				SourceID:       source.UID,
				ProjectID:      source.ProjectID,
				EndpointID:     convoyEvent.EndpointID,
				EventType:      convoyEvent.EventType,
				Data:           convoyEvent.Data,
				CustomHeaders:  headers,
				IdempotencyKey: convoyEvent.IdempotencyKey,
			},
			CreateSubscription: !util.IsStringEmpty(convoyEvent.EndpointID),
		}

		if util.IsStringEmpty(ce.Params.EndpointID) {
			err := fmt.Errorf("the payload for %s with id (%s) doesn't include an endpoint id, please refer to the documentation or"+
				" use transfrom functions to properly format it, got: %+v", source.Name, source.UID, convoyEvent)
			return err
		}

		eventByte, err := msgpack.EncodeMsgPack(ce)
		if err != nil {
			return err
		}

		job := &queue.Job{
			ID:      ce.Params.UID,
			Payload: eventByte,
		}

		// write to our queue if it's a normal event
		err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			return err
		}
	case "broadcast":
		broadcastEvent := models.BroadcastEvent{
			ProjectID:      source.ProjectID,
			SourceID:       source.UID,
			EventType:      convoyEvent.EventType,
			Data:           convoyEvent.Data,
			CustomHeaders:  headers,
			IdempotencyKey: convoyEvent.IdempotencyKey,
		}

		eventByte, err := msgpack.EncodeMsgPack(broadcastEvent)
		if err != nil {
			return err
		}

		job := &queue.Job{
			ID:      ulid.Make().String(),
			Payload: eventByte,
		}

		// write to our queue if it's a broadcast event
		err = i.queue.Write(convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			return err
		}
	default:
		err := fmt.Errorf("%s isn't a valid pubsub message type, it should be one of single and broadcast", messageType)
		log.Error(err)
		return err
	}

	return nil
}

func mergeHeaders(dest map[string]string, src map[string]string) {
	for k, v := range src {
		if _, ok := dest[k]; ok {
			continue
		}

		dest[k] = v
	}

	_, ok := dest[ConvoyMessageTypeHeader]
	if !ok {
		// the message type header wasn't found, set it to a default value
		dest[ConvoyMessageTypeHeader] = "single"
	}
}

func handlePanic(source *datastore.Source) {
	if err := recover(); err != nil {
		log.Error(fmt.Errorf("sourceID: %s, Error: %s", source.UID, err))
	}
}
