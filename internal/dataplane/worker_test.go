package dataplane

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/broker"
)

func TestWorkerQueueNamesAreProviderNeutral(t *testing.T) {
	names, err := broker.QueueNames(config.DefaultExecutionMode)
	assert.NoError(t, err)
	assert.Contains(t, names, string(convoy.EventQueue))
	assert.Contains(t, names, string(convoy.RetryEventQueue))
}
