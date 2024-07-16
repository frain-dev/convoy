package testcon

import (
	"context"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func (i *IntegrationTestSuite) Test_DirectEvent_Success_AllSubscriptions() {
	ctx := context.Background()
	t := i.T()
	var ports = []int{9909}

	c, done := i.initAndStartServers(ports, 2)

	endpoint := createEndpoints(t, ctx, c, ports, i.DefaultOrg.OwnerID)[0]

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"*"})

	traceId, secondTraceId := "event-direct-all-0-"+ulid.Make().String(), "event-direct-all-1-"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "any.event", traceId, ""))
	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "any.other.event", secondTraceId, ""))

	assertEventCameThrough(t, done, []*convoy.EndpointResponse{endpoint}, []string{traceId, secondTraceId})
}

func (i *IntegrationTestSuite) Test_DirectEvent_Success_MustMatchSubscription() {
	ctx := context.Background()
	t := i.T()
	var ports = []int{9910}

	c, done := i.initAndStartServers(ports, 1)

	endpoint := createEndpoints(t, ctx, c, ports, i.DefaultOrg.OwnerID)[0]

	createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"invoice.created"})

	traceId, secondTraceId := "event-direct-some-0-"+ulid.Make().String(), "event-direct-some-0"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "mismatched.event", traceId, ""))
	require.NoError(t, sendEvent(ctx, c, "direct", endpoint.UID, "invoice.created", secondTraceId, ""))

	assertEventCameThrough(t, done, []*convoy.EndpointResponse{endpoint}, []string{secondTraceId})
}
