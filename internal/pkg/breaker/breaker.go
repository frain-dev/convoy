package breaker

import (
	"context"
	"time"

	"github.com/cep21/circuit/v4"
	"github.com/cep21/circuit/v4/closers/hystrix"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

// A Circuit that syncs with the database.
type Circuit struct {
	cb       *circuit.Circuit
	id       string
	endpoint *datastore.Endpoint
}

func NewCircuit(cb *circuit.Circuit, id string) *Circuit {
	// retrieve endpoint from DB.

	return &Circuit{
		cb: cb,
		id: id,
	}
}

func (c *Circuit) IsOpen() bool {
	return c.cb.IsOpen()
}

func (c *Circuit) OpenCircuit(ctx context.Context) {
	c.cb.OpenCircuit(ctx)
}

func (c *Circuit) Run(ctx context.Context, runFunc func(context.Context) error) error {
	return c.cb.Run(ctx, runFunc)
}

type EndpointConfig struct {
	UID            string
	Duration       int64
	ErrorThreshold float64
}

type CircuitManager interface {
	Get(endpointID string) *Circuit
	CreateCircuit(*datastore.Endpoint, datastore.EndpointRepository) (*Circuit, error)
}

type Manager struct {
	cm *circuit.Manager
}

func NewManager() *Manager {
	m := circuit.Manager{}
	return &Manager{&m}
}

func (m *Manager) Get(endpointID string) *Circuit {
	cb := m.cm.GetCircuit(endpointID)
	if cb != nil {
		return NewCircuit(cb, endpointID)
	}

	return nil
}

// Use this to create the circuit for each endpoint.
// m.CreateCircuit(EndpointConfig{})
func (m *Manager) CreateCircuit(endpoint *datastore.Endpoint, repo datastore.EndpointRepository) (*Circuit, error) {
	// TODO(subomi): add config to persist status once the circuit trips!
	cfg := circuit.Config{
		General: circuit.GeneralConfig{
			OpenToClosedFactory: m.createCloser(endpoint),
			ClosedToOpenFactory: m.createOpener(endpoint),
		},
		Metrics: circuit.MetricsCollectors{
			Circuit: []circuit.Metrics{
				&EndpointStateManager{
					endpoint:     endpoint,
					endpointRepo: repo,
				},
			},
		},
	}

	cb, err := m.cm.CreateCircuit(endpoint.UID, cfg)
	if err != nil {
		return nil, err
	}

	return NewCircuit(cb, endpoint.UID), nil
}

func (m *Manager) createCloser(endpoint *datastore.Endpoint) func() circuit.OpenToClosed {
	cc := hystrix.ConfigureCloser{
		SleepWindow:      time.Duration((time.Duration(endpoint.CircuitBreaker.Duration) * time.Second).Milliseconds()),
		HalfOpenAttempts: 1,
	}

	return hystrix.CloserFactory(cc)
}

func (m *Manager) createOpener(endpoint *datastore.Endpoint) func() circuit.ClosedToOpen {
	co := hystrix.ConfigureOpener{
		ErrorThresholdPercentage: int64(endpoint.CircuitBreaker.ErrorThreshold),
		RollingDuration:          time.Duration(endpoint.CircuitBreaker.Duration),
	}

	return hystrix.OpenerFactory(co)
}

type EndpointStateManager struct {
	endpoint     *datastore.Endpoint
	endpointRepo datastore.EndpointRepository
}

// TODO(subomi): Add notification logic.
func (es *EndpointStateManager) Closed(ctx context.Context, now time.Time) {
	err := es.endpointRepo.UpdateEndpointStatus(ctx, es.endpoint.ProjectID, es.endpoint.UID, datastore.ActiveEndpointStatus)
	if err != nil {
		log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
	}
}
func (es *EndpointStateManager) Opened(ctx context.Context, now time.Time) {
	err := es.endpointRepo.UpdateEndpointStatus(ctx, es.endpoint.ProjectID, es.endpoint.UID, datastore.InactiveEndpointStatus)
	if err != nil {
		log.WithError(err).Error("Failed to deactivate endpoint after failed retry")
	}

}
