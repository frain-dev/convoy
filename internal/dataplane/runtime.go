package dataplane

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
)

type Runtime struct {
	deps     RuntimeOpts
	cfg      config.Configuration
	interval int
	worker   *Worker
}

func New(ctx context.Context, opts RuntimeOpts, cfg config.Configuration, interval int) (*Runtime, error) {
	worker, err := NewWorker(ctx, opts, cfg)
	if err != nil {
		return nil, fmt.Errorf("error initializing data plane worker component: %w", err)
	}

	return &Runtime{
		deps:     opts,
		cfg:      cfg,
		interval: interval,
		worker:   worker,
	}, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	go memorystore.DefaultStore.Sync(ctx, r.interval)

	workerReady := make(chan struct{})
	workerErr := make(chan error, 1)

	go func() {
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
		r.deps.Logger.Info("Worker is ready")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("worker failed to become ready within 30 seconds")
	}

	if err := StartIngest(ctx, r.deps, r.cfg); err != nil {
		return fmt.Errorf("error starting data plane ingest component: %w", err)
	}

	if err := StartServer(r.deps, r.cfg); err != nil {
		return fmt.Errorf("error starting data plane server component: %w", err)
	}

	<-ctx.Done()

	return ctx.Err()
}
