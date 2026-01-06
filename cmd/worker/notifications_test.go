package worker

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type testQueue struct{ wrote []*queue.Job }

func (tq *testQueue) Write(_ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	tq.wrote = append(tq.wrote, job)
	return nil
}

func (tq *testQueue) WriteWithoutTimeout(_ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	tq.wrote = append(tq.wrote, job)
	return nil
}

func (tq *testQueue) Options() queue.QueueOptions { return queue.QueueOptions{} }

func TestEnqueueCircuitBreakerEmails(t *testing.T) {
	q := &testQueue{}
	lo := log.NewLogger(nil)

	project := &datastore.Project{Name: "P1", LogoURL: "http://logo"}
	endpoint := &datastore.Endpoint{Name: "E1", Url: "http://e1", SupportEmail: "ep@x.y", FailureRate: 42}

	err := EnqueueCircuitBreakerEmails(q, lo, project, endpoint, "owner@x.y")
	require.NoError(t, err)
	require.Len(t, q.wrote, 2) // endpoint + owner

	// When no support email
	q2 := &testQueue{}
	endpoint2 := &datastore.Endpoint{Name: "E2", Url: "http://e2", SupportEmail: ""}
	err = EnqueueCircuitBreakerEmails(q2, lo, project, endpoint2, "owner@x.y")
	require.NoError(t, err)
	require.Len(t, q2.wrote, 1)
}
