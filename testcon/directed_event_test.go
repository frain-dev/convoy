package testcon

import (
	"context"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"sync/atomic"
)

func (i *IntegrationTestSuite) Test_DirectEvent_Success_AllSubscriptions() {
	ctx := context.Background()
	t := i.T()
	const port = 9909

	c, done := i.initAndStartServer(port, 2)

	endpoint := createEndpoint(t, ctx, c, port)

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"*"})

	traceId, secondTraceId := "event-"+ulid.Make().String(), "event-"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "any.event", traceId))
	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "any.other.event", secondTraceId))

	assertEventCameThrough(t, done, endpoint.TargetUrl, traceId, secondTraceId)
}

func (i *IntegrationTestSuite) Test_DirectEvent_Success_MustMatchSubscription() {
	ctx := context.Background()
	t := i.T()
	const port = 9910

	c, done := i.initAndStartServer(port, 1)

	endpoint := createEndpoint(t, ctx, c, port)

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"invoice.created"})

	traceId, secondTraceId := "event-"+ulid.Make().String(), "event-"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "mismatched.event", traceId))
	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "invoice.created", secondTraceId))

	assertEventCameThrough(t, done, endpoint.TargetUrl, secondTraceId)
}

func (i *IntegrationTestSuite) initAndStartServer(port int, eventCount int64) (*convoy.Client, chan bool) {
	baseURL := "http://localhost:5015/api/v1"
	c := convoy.New(baseURL, i.APIKey, i.DefaultProject.UID)

	done := make(chan bool, 1)

	var counter atomic.Int64
	counter.Store(eventCount)

	go startHTTPServer(done, &counter, port)

	return c, done
}
