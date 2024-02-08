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

func (c *Circuit) Config() circuit.Config {
	return c.cb.Config()
}

func (c *Circuit) SetConfig(cfg circuit.Config) {
	c.cb.SetConfigThreadSafe(cfg)
}

type CircuitManager interface {
	Get(endpointID string) *Circuit
	CreateCircuit(*datastore.Endpoint) (*Circuit, error)
}

type Manager struct {
	cm *circuit.Manager

	projectRepo  datastore.ProjectRepository
	endpointRepo datastore.EndpointRepository
}

func NewManager(ctx context.Context, projectRepo datastore.ProjectRepository, endpointRepo datastore.EndpointRepository) (*Manager, error) {
	m := &Manager{
		cm:           &circuit.Manager{},
		projectRepo:  projectRepo,
		endpointRepo: endpointRepo,
	}

	if err := m.init(ctx); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) DatabaseEndpointSyncer(ctx context.Context, interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	for {
		select {
		case <-ticker.C:
			// retrieve the endpoint & check if config changed.
			endpoints, err := m.retrieveAllEndpoints(ctx)
			if err != nil {
				continue
			}
			for _, endpoint := range endpoints {
				cb := m.Get(endpoint.UID)
				if cb == nil {
					_, err := m.CreateCircuit(&endpoint)
					if err != nil {
						log.WithError(err).Errorf("Failed to initialise circuit for %s", endpoint.UID)
					}
				}

				// update circuit status.
				if cb.Config().General.Disabled != endpoint.DisableEndpoint {
					cfg := m.retrieveDefaultCircuitConfig(&endpoint)
					cfg.General.Disabled = endpoint.DisableEndpoint
					cb.SetConfig(*m.retrieveDefaultCircuitConfig(cb.endpoint))
				}
			}
		case <-ctx.Done():
			// Stop the ticker
			ticker.Stop()

			return
		}
	}
}

func (m *Manager) init(ctx context.Context) error {
	endpoints, err := m.retrieveAllEndpoints(ctx)
	if err != nil {
		return err
	}

	for _, endpoint := range endpoints {
		_, err := m.CreateCircuit(&endpoint)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) retrieveAllEndpoints(ctx context.Context) ([]datastore.Endpoint, error) {
	projects, err := m.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		count, err := m.endpointRepo.CountProjectEndpoints(ctx, project.UID)
		if err != nil {
			return nil, err
		}

		endpoints, _, err := m.endpointRepo.LoadEndpointsPaged(ctx, project.UID, &datastore.Filter{}, datastore.Pageable{PerPage: int(count)})
		if err != nil {
			return nil, err
		}

		for _, endpoint := range endpoints {
			if endpoint.CircuitBreaker.Duration == 0 {
				endpoint.CircuitBreaker.Duration = project.Config.CircuitBreaker.Duration
			}

			if endpoint.CircuitBreaker.ErrorThreshold == 0 {
				endpoint.CircuitBreaker.ErrorThreshold = project.Config.CircuitBreaker.ErrorThreshold
			}
		}

		return endpoints, nil
	}
	return nil, nil
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
func (m *Manager) CreateCircuit(endpoint *datastore.Endpoint) (*Circuit, error) {
	cb, err := m.cm.CreateCircuit(endpoint.UID, *m.retrieveDefaultCircuitConfig(endpoint))
	if err != nil {
		return nil, err
	}

	return NewCircuit(cb, endpoint.UID), nil
}

func (m *Manager) retrieveDefaultCircuitConfig(endpoint *datastore.Endpoint) *circuit.Config {
	return &circuit.Config{
		General: circuit.GeneralConfig{
			Disabled:            endpoint.DisableEndpoint,
			OpenToClosedFactory: m.createCloser(endpoint),
			ClosedToOpenFactory: m.createOpener(endpoint),
		},
		Metrics: circuit.MetricsCollectors{
			Circuit: []circuit.Metrics{
				&EndpointStateManager{
					endpoint:     endpoint,
					endpointRepo: m.endpointRepo,
				},
			},
		},
	}

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

func (m *Manager) retrieveEndpoints() error {
	go func() {
		// polls for endpoints.
	}()

	return nil
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
