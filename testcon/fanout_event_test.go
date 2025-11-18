//go:build docker_testcon
// +build docker_testcon

package testcon

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

func (d *DockerE2EIntegrationTestSuite) Test_FanOutEvent_Success_AllSubscriptions() {
	ctx := context.Background()
	t := d.T()
	ownerId := d.DefaultOrg.OwnerID + "_2"

	var ports = []int{9911, 9912, 9913}

	c, done := d.initAndStartServers(t, ports, 3*2)

	endpoints := createEndpoints(t, ctx, c, ports, ownerId)

	traceIds := make([]string, 0)
	for _, endpoint := range endpoints {
		createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"*"})
	}

	traceId, secondTraceId := "event-fan-out-all-0-"+ulid.Make().String(), "event-fan-out-all-1-"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "fan-out", "", "any.event", traceId, ownerId))
	require.NoError(t, sendEvent(ctx, c, "fan-out", "", "any.other.event", secondTraceId, ownerId))

	traceIds = append(traceIds, traceId, secondTraceId)

	assertEventCameThrough(t, done, endpoints, traceIds, []string{})
}

func (d *DockerE2EIntegrationTestSuite) Test_FanOutEvent_Success_MustMatchSubscription() {
	ctx := context.Background()
	t := d.T()
	ownerID := d.DefaultOrg.OwnerID + "_3"

	var ports = []int{9914, 9915, 9916}

	c, done := d.initAndStartServers(t, ports, 3*1) // 3 endpoints, 1 event each

	endpoints := createEndpoints(t, ctx, c, ports, ownerID)

	traceIds := make([]string, 0)
	negativeTraceIds := make([]string, 0)
	for _, endpoint := range endpoints {
		createMatchingSubscriptions(t, ctx, c, endpoint.UID, []string{"invoice.fan-out.created"})
	}

	traceId, secondTraceId := "event-fan-out-some-0-"+ulid.Make().String(), "event-fan-out-some-1-"+ulid.Make().String()

	require.NoError(t, sendEvent(ctx, c, "fan-out", "", "mismatched.event.dont.fan.out", traceId, ownerID))
	require.NoError(t, sendEvent(ctx, c, "fan-out", "", "invoice.fan-out.created", secondTraceId, ownerID))

	traceIds = append(traceIds, secondTraceId)
	negativeTraceIds = append(negativeTraceIds, traceId)

	assertEventCameThrough(t, done, endpoints, traceIds, negativeTraceIds)
}

func (d *DockerE2EIntegrationTestSuite) initAndStartServers(t *testing.T, ports []int, eventCount int64) (*convoy.Client, chan bool) {
	return d.initAndStartServersWithType(t, ports, eventCount, "regular")
}

func (d *DockerE2EIntegrationTestSuite) initAndStartServersWithType(t *testing.T, ports []int, eventCount int64, serverType string) (*convoy.Client, chan bool) {
	baseURL := fmt.Sprintf("http://%s:5015/api/v1", GetOutboundIP().String())
	c := convoy.New(baseURL, d.APIKey, d.DefaultProject.UID)

	done := make(chan bool, 1)

	var counter atomic.Int64
	counter.Store(eventCount)

	for _, port := range ports {
		if serverType == "form" {
			go startFormHTTPServer(done, &counter, port)
		} else {
			go startHTTPServer(done, &counter, port)
		}
	}

	// Give servers a moment to start - endpoint creation retry will handle server availability
	time.Sleep(100 * time.Millisecond)

	return c, done
}
