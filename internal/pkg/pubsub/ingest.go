package pubsub

import (
	"context"
	"encoding/json"
	"errors"
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
	ctx        context.Context
	cancelFunc context.CancelFunc
	ticker     *time.Ticker
	queue      queue.Queuer
	sources    map[string]*PubSubSource
	table      *memorystore.Table
	log        log.StdLogger
}

func NewIngest(ctx context.Context, table *memorystore.Table, queue queue.Queuer, log log.StdLogger) (*Ingest, error) {
	ctx, cancel := context.WithCancel(ctx)
	i := &Ingest{
		ctx:        ctx,
		cancelFunc: cancel,
		queue:      queue,
		log:        log,
		table:      table,
		ticker:     time.NewTicker(time.Duration(1) * time.Second),
	}

	// initialise ingest sources
	for _, row := range table.GetAll() {
		r := row.Value()
		s, ok := r.(*datastore.Source)
		if !ok {
			return nil, errors.New("invalid source in memory source table")
		}

		ps, err := NewPubSubSource(ctx, s, i.handler, i.log)
		if err != nil {
			return nil, err
		}

		i.sources[generateSourceKey(s)] = ps
	}

	return i, nil
}

func (i *Ingest) Run() {
	// loop through the tables in an interval
	// find the diff
	// cancel the ctx
	// start afresh.

	for {
		select {
		case <-i.ticker.C:
			log.Info("refreshing runner...")
			err := i.run()
			if err != nil {
				i.log.WithError(err).Error("ingest runner failed")
			}

		case <-i.ctx.Done():
			// stop ticker.
			i.ticker.Stop()

			// cancel all sources.
			i.cancelFunc()
		}
	}
}

func (i *Ingest) Stop() {
	i.cancelFunc()
}

func (i *Ingest) getSourceKeys() []string {
	var s []string
	for _, v := range i.sources {
		s = append(s, v.hash)
	}

	return s
}

func (i *Ingest) run() error {
	ctx := context.WithValue(i.ctx, ingestCtx, nil)

	i.log.Info("refreshing runner...")

	// cancel
	staleRows := util.Difference(i.table.GetKeys(), i.getSourceKeys())
	for _, s := range staleRows {
		for _, ps := range i.sources {
			if s == ps.hash {
				// cancel context.
				ps.Stop()
			}
		}
	}

	// start new & updated.
	newSourceKeys := util.Difference(i.getSourceKeys(), i.table.GetKeys())
	for _, s := range newSourceKeys {
		for _, t := range i.table.GetAll() {
			ss, ok := t.Value().(*datastore.Source)
			if !ok {
				return errors.New("invalid store in memory table")
			}

			// start new runners.
			ps, err := NewPubSubSource(ctx, ss, i.handler, i.log)
			if err != nil {
				return err
			}

			ps.Start()
			i.sources[s] = ps
		}

	}
	return nil
}

func (i *Ingest) handler(_ context.Context, source *datastore.Source, msg string) error {
	ev := struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		CustomHeaders  map[string]string `json:"custom_headers"`
		IdempotencyKey string            `json:"idempotency_key"`
	}{}

	if err := json.Unmarshal([]byte(msg), &ev); err != nil {
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

	err = i.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		return err
	}

	return nil
}
