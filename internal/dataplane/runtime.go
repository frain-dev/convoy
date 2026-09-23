package dataplane

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/broker"
	"github.com/frain-dev/convoy/internal/pkg/indexes"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/partitions"
)

type Runtime struct {
	opts     RuntimeOpts
	cfg      config.Configuration
	interval int
	worker   *Worker
	source   *Worker
}

func New(ctx context.Context, opts RuntimeOpts, cfg config.Configuration, interval int) (*Runtime, error) {
	worker, err := NewWorker(ctx, opts, cfg)
	if err != nil {
		return nil, fmt.Errorf("error initializing data plane worker component: %w", err)
	}

	var source *Worker
	if opts.Broker != nil && opts.Broker.Source != nil {
		sourceOpts := opts
		sourceOpts.SourceDrain = true
		sourceOpts.Broker = opts.Broker.Source
		sourceOpts.Queue = sourceOpts.Broker.WorkerQueue
		sourceOpts.Cache = sourceOpts.Broker.Cache
		sourceOpts.Rate = sourceOpts.Broker.RateLimiter
		sourceCfg := cfg
		sourceCfg.QueueProvider = cfg.PreviousQueueProvider
		if err := broker.AllowPostgresQueue(sourceCfg, opts.Licenser); err != nil {
			return nil, err
		}
		source, err = NewWorker(ctx, sourceOpts, sourceCfg)
		if err != nil {
			return nil, fmt.Errorf("initialize previous queue executor: %w", err)
		}
	}
	return &Runtime{
		source:   source,
		opts:     opts,
		cfg:      cfg,
		interval: interval,
		worker:   worker,
	}, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()

	go memorystore.DefaultStore.Sync(ctx, r.interval)

	workerReady := make(chan struct{})
	workerErr := make(chan error, 1)

	if r.source != nil {
		sourceReady := make(chan struct{})
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := r.source.Run(ctx, sourceReady); err != nil && ctx.Err() == nil {
				workerErr <- err
			}
		}()
		select {
		case <-sourceReady:
		case err := <-workerErr:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		if err := r.worker.Run(ctx, workerReady); err != nil {
			if ctx.Err() == nil {
				workerErr <- err
			}
		}
	}()

	select {
	case err := <-workerErr:
		return fmt.Errorf("worker failed to start: %w", err)
	case <-workerReady:
		r.opts.Logger.Info("Worker is ready")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("worker failed to become ready within 30 seconds")
	}

	// Fail open: queries seq-scan until owed indexes are valid. Do not block
	// ingest. Orphan invalid indexes are adopted into dropped_indexes first.
	if n, err := indexes.Adopt(ctx, r.opts.DB.GetConn()); err != nil {
		r.opts.Logger.Error("invalid index adoption failed", "error", err.Error())
	} else if n > 0 {
		r.opts.Logger.Info("adopted invalid indexes for rebuild", "count", n)
	}
	p := partitions.New(r.opts.DB, r.opts.Logger)
	p.StartQueuedDroppedIndexes(ctx)

	if err := StartIngest(ctx, r.opts, r.cfg); err != nil {
		return fmt.Errorf("error starting data plane ingest component: %w", err)
	}

	if err := StartServer(r.opts, r.cfg); err != nil {
		return fmt.Errorf("error starting data plane server component: %w", err)
	}

	<-ctx.Done()

	return ctx.Err()
}
