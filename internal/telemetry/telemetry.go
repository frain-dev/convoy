package telemetry

import (
	"context"
	"io"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

const (
	DailyEventCount         string = "Daily Event Count"
	EventCounter            string = "Event Counter"
	DailyActiveProjectCount string = "Daily Active Project Count"
)

type Backend interface {
	io.Closer
	Capture(ctx context.Context, metric Metric) error
	Identify(ctx context.Context, instanceID string) error
}

type metricName string

type Metric struct {
	Name       metricName
	InstanceID string
	Alias      string
	Count      uint64
	Version    string
}

type Tracker interface {
	track(ctx context.Context, instanceID string) (Metric, error)
}

type TelemetryOption func(*Telemetry)

func OptionTracker(tr Tracker) func(*Telemetry) {
	return func(t *Telemetry) {
		t.trackers = append(t.trackers, tr)
	}
}

func OptionBackend(b Backend) func(*Telemetry) {
	return func(t *Telemetry) {
		t.backends = append(t.backends, b)
	}
}

type Telemetry struct {
	config   *datastore.Configuration
	backends []Backend
	trackers []Tracker
	Logger   *log.Logger
}

func NewTelemetry(log *log.Logger, config *datastore.Configuration, opts ...TelemetryOption) *Telemetry {
	t := &Telemetry{
		config: config,
		Logger: log,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Identify on startup: telemetry.Identify(instanceId)
func (t *Telemetry) Identify(ctx context.Context, instanceID string) error {
	isEnabled := t.config.IsAnalyticsEnabled
	if !isEnabled {
		return nil
	}

	for i := range t.backends {
		go func(b Backend) {
			err := b.Identify(ctx, instanceID)
			if err != nil {
				t.Logger.Error(err)
			}

			err = b.Close()
			if err != nil {
				t.Logger.Error(err)
			}
		}(t.backends[i])
	}

	return nil
}

// Capture at an interval: telemetry.Track()
func (t *Telemetry) Capture(ctx context.Context) error {
	isEnabled := t.config.IsAnalyticsEnabled
	if !isEnabled {
		return nil
	}

	var metrics []Metric

	// generate metrics
	for _, tr := range t.trackers {
		m, err := tr.track(ctx, t.config.UID)
		if err != nil {
			// what do we do when one Tracker fails?
			t.Logger.Error(err)
			continue
		}

		metrics = append(metrics, m)
	}

	for i := range t.backends {
		go func(b Backend) {
			for m := range metrics {
				err := b.Capture(ctx, metrics[m])
				if err != nil {
					t.Logger.Error(err)
				}
			}

			err := b.Close()
			if err != nil {
				t.Logger.Error(err)
			}
		}(t.backends[i])
	}

	return nil
}
