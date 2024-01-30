package telemetry

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

const (
	posthogKey              string = ""
	DailyEventCount         string = "Daily Event Count"
	DailyOrganisationCount  string = "Daily Organization Count"
	DailyProjectCount       string = "Daily Project Count"
	DailyActiveProjectCount string = "Daily Active Project Count"
	DailyUserCount          string = "Daily User Count"
)

type Backend interface {
	Capture(ctx context.Context, metric metric) error
	Identify(ctx context.Context, instanceID string) error
}

type metricName string

type metric struct {
	Name       metricName
	InstanceID string
	Alias      string
	Count      uint64
}

type tracker interface {
	Track(ctx context.Context, instanceID string) (metric, error)
}

type TelemetryOption func(*Telemetry)

func OptionTracker(tr tracker) func(*Telemetry) {
	return func(t *Telemetry) {
		t.trackers = append(t.trackers, tr)
	}
}

type Telemetry struct {
	config   *datastore.Configuration
	backends []Backend
	trackers []tracker
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

// on startup: telemetry.Identify(instanceId)
func (t *Telemetry) Identify(ctx context.Context, instanceID string) error {
	for _, b := range t.backends {
		go func(b Backend) {
			err := b.Identify(ctx, instanceID)
			if err != nil {
				t.Logger.Error(err)
			}
		}(b)
	}

	return nil
}

// at an interval: telemetry.Track()
func (t *Telemetry) Capture(ctx context.Context) error {
	isEnabled := t.config.IsAnalyticsEnabled
	if !isEnabled {
		return nil
	}

	var metrics []metric

	// generate metrics
	for _, tr := range t.trackers {
		metric, err := tr.Track(ctx, t.config.UID)
		if err != nil {
			// what do we do when one tracker fails?
			continue
		}

		metrics = append(metrics, metric)
	}

	for _, b := range t.backends {
		go func(b Backend) {
			for _, m := range metrics {
				err := b.Capture(ctx, m)
				if err != nil {
					t.Logger.Error(err)
				}
			}
		}(b)
	}

	return nil
}
