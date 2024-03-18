package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dop251/goja"
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

func (i *Ingest) handler(_ context.Context, source *datastore.Source, msg string) error {
	// unmarshal to an interface{} struct
	var raw any
	if err := json.Unmarshal([]byte(msg), &raw); err != nil {
		return err
	}

	log.Infof("raw: %+v\n", raw)

	type ConvoyEvent struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		CustomHeaders  map[string]string `json:"custom_headers"`
		IdempotencyKey string            `json:"idempotency_key"`
	}

	var payload any
	if source.Function.Ptr() != nil && !util.IsStringEmpty(source.Function.String) {
		t := transform.NewTransformer(goja.New())
		p, _, err := t.Transform(source.Function.String, raw)
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

	log.Infof("payload: %+v\n", payload)

	var ev ConvoyEvent
	if err := json.Unmarshal(pBytes, &ev); err != nil {
		return err
	}

	ce := task.CreateEvent{
		Params: task.CreateEventTaskParams{
			UID:            ulid.Make().String(),
			SourceID:       source.UID,
			ProjectID:      source.ProjectID,
			EndpointID:     ev.EndpointID,
			EventType:      ev.EventType,
			Data:           ev.Data,
			CustomHeaders:  ev.CustomHeaders,
			IdempotencyKey: ev.IdempotencyKey,
		},
		CreateSubscription: !util.IsStringEmpty(ev.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(ce)
	if err != nil {
		return err
	}

	job := &queue.Job{
		ID:      ce.Params.UID,
		Payload: eventByte,
		Delay:   0,
	}

	// write to our queue
	err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		return err
	}

	return nil
}
