package testcon

import (
	"context"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"sync/atomic"
)

func (i *IntegrationTestSuite) Test_FanOutEvent_Success_AllSubscriptions() {
	ctx := context.Background()
	t := i.T()

	var ports = []int{9911, 9912, 9913}

	c, done := i.initAndStartServers(ports, 3*2*2) // 3 endpoints, 2 events each, 2 fan-out operations

	endpoints := createEndpoints(t, ctx, c, ports, i.DefaultOrg.OwnerID)

	traceIds := make([]string, 0)
	for _, endpoint := range endpoints {
		createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"*"})

		traceId, secondTraceId := "event-fan-out-all-0"+ulid.Make().String(), "event-fan-out-all-1"+ulid.Make().String()

		require.NoError(t, sendEvent(ctx, c, "fan-out", endpoint.UID, "any.event", traceId, i.DefaultOrg.OwnerID))
		require.NoError(t, sendEvent(ctx, c, "fan-out", endpoint.UID, "any.other.event", secondTraceId, i.DefaultOrg.OwnerID))

		traceIds = append(traceIds, traceId, secondTraceId)
	}

	assertEventCameThrough(t, done, endpoints, traceIds)
}

func (i *IntegrationTestSuite) Test_FanOutEvent_Success_MustMatchSubscription() {
	ctx := context.Background()
	t := i.T()

	var ports = []int{9914, 9915, 9916}

	c, done := i.initAndStartServers(ports, 3*1) // 3 endpoints, 1 event each

	endpoints := createEndpoints(t, ctx, c, ports, i.DefaultOrg.OwnerID)

	traceIds := make([]string, 0)
	for _, endpoint := range endpoints {
		createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"invoice.fan-out.created"})

		traceId, secondTraceId := "event-fan-out-some-0"+ulid.Make().String(), "event-fan-out-some-1"+ulid.Make().String()

		require.NoError(t, sendEvent(ctx, c, "fan-out", endpoint.UID, "mismatched.event", traceId, i.DefaultOrg.OwnerID))
		require.NoError(t, sendEvent(ctx, c, "fan-out", endpoint.UID, "invoice.fan-out.created", secondTraceId, i.DefaultOrg.OwnerID))

		traceIds = append(traceIds, secondTraceId)
	}

	assertEventCameThrough(t, done, endpoints, traceIds)
}

func (i *IntegrationTestSuite) initAndStartServers(ports []int, eventCount int64) (*convoy.Client, *chan bool) {
	baseURL := "http://localhost:5015/api/v1"
	c := convoy.New(baseURL, i.APIKey, i.DefaultProject.UID)

	done := make(chan bool, 1)

	var counter atomic.Int64
	counter.Store(eventCount)

	for _, port := range ports {
		go startHTTPServer(done, &counter, port)
	}

	return c, &done
}
