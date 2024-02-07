package breaker

import (
	"context"
	"time"

	"github.com/cep21/circuit/v4"
	"github.com/cep21/circuit/v4/closers/hystrix"
)

type Circuit struct {
	c *circuit.Circuit
}

func NewCircuit() *Circuit {
	return &Circuit{}
}

func (c *Circuit) IsOpen() bool {
	return c.c.IsOpen()
}

func (c *Circuit) OpenCircuit(ctx context.Context) {
	c.c.OpenCircuit(ctx)
}

func (c *Circuit) Run(ctx context.Context, runFunc func(context.Context) error) error {
	return c.c.Run(ctx, runFunc)
}

type EndpointConfig struct {
	UID            string
	Duration       int64
	ErrorThreshold float64
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
		return &Circuit{cb}
	}

	return nil
}

// Use this to create the circuit for each endpoint.
// m.CreateCircuit(EndpointConfig{})
func (m *Manager) CreateCircuit(e *EndpointConfig) (*Circuit, error) {
	cfg := circuit.Config{
		General: circuit.GeneralConfig{
			OpenToClosedFactory: m.createCloser(e),
			ClosedToOpenFactory: m.createOpener(e),
		},
	}

	cc, err := m.cm.CreateCircuit(e.UID, cfg)
	if err != nil {
		return nil, err
	}

	return &Circuit{cc}, nil
}

func (m *Manager) createCloser(e *EndpointConfig) func() circuit.OpenToClosed {
	cc := hystrix.ConfigureCloser{
		SleepWindow:      time.Duration((time.Duration(e.Duration) * time.Second).Milliseconds()),
		HalfOpenAttempts: 1,
	}

	return hystrix.CloserFactory(cc)
}

func (m *Manager) createOpener(e *EndpointConfig) func() circuit.ClosedToOpen {
	co := hystrix.ConfigureOpener{
		ErrorThresholdPercentage: int64(e.ErrorThreshold),
	}

	return hystrix.OpenerFactory(co)
}
