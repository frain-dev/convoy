package task

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type stubConfigRepo struct {
	cfg *datastore.Configuration
	err error
}

func (s *stubConfigRepo) CreateConfiguration(context.Context, *datastore.Configuration) error {
	return nil
}
func (s *stubConfigRepo) LoadConfiguration(context.Context) (*datastore.Configuration, error) {
	return s.cfg, s.err
}
func (s *stubConfigRepo) UpdateConfiguration(context.Context, *datastore.Configuration) error {
	return nil
}

type stubRetentioner struct {
	called bool
}

func (s *stubRetentioner) Perform(context.Context) error {
	s.called = true
	return nil
}
func (s *stubRetentioner) Start(context.Context, time.Duration) {}

type passthroughLocker struct{}

func (passthroughLocker) WithLock(ctx context.Context, _ string, _ time.Duration, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestRetentionPolicies_SkipsWhenDisabled(t *testing.T) {
	ret := &stubRetentioner{}
	cfgRepo := &stubConfigRepo{
		cfg: &datastore.Configuration{
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{
				Period:  "720h",
				Enabled: false,
			},
		},
	}

	task := asynq.NewTask(string(convoy.RetentionPolicies), nil)
	err := RetentionPolicies(passthroughLocker{}, cfgRepo, ret, log.New("test", log.LevelInfo))(context.Background(), task)
	require.NoError(t, err)
	require.False(t, ret.called)
}

func TestRetentionPolicies_RunsWhenEnabled(t *testing.T) {
	ret := &stubRetentioner{}
	cfgRepo := &stubConfigRepo{
		cfg: &datastore.Configuration{
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{
				Period:  "720h",
				Enabled: true,
			},
		},
	}

	task := asynq.NewTask(string(convoy.RetentionPolicies), nil)
	err := RetentionPolicies(passthroughLocker{}, cfgRepo, ret, log.New("test", log.LevelInfo))(context.Background(), task)
	require.NoError(t, err)
	require.True(t, ret.called)
}
