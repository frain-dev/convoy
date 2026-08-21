package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoy "github.com/frain-dev/convoy-go/v2"
)

type pagedEvents struct {
	Content []struct {
		UID       string          `json:"uid"`
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	} `json:"content"`
}

// TestE2E_EventSearch_ExactJSONFilterReadsThroughAPI drives the licensed event
// search over HTTP against events that were really ingested, so the filter runs
// against payloads the writer stored rather than rows a test inserted.
func TestE2E_EventSearch_ExactJSONFilterReadsThroughAPI(t *testing.T) {
	env := SetupE2E(t)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	port := 19913
	StartMockWebhookServer(t, manifest, done, &counter, port)

	client := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	ownerID := "search-" + ulid.Make().String()
	endpoint := CreateEndpointViaSDK(t, client, port, ownerID)
	CreateSubscriptionViaSDK(t, client, endpoint.UID, []string{"*"})

	wanted := ulid.Make().String()
	require.NoError(t, client.Events.Create(t.Context(), &convoy.CreateEventRequest{
		EndpointID:     endpoint.UID,
		EventType:      "search.match",
		IdempotencyKey: ulid.Make().String(),
		Data:           []byte(`{"tenant":"` + wanted + `","plan":"scale"}`),
	}))
	require.NoError(t, client.Events.Create(t.Context(), &convoy.CreateEventRequest{
		EndpointID:     endpoint.UID,
		EventType:      "search.other",
		IdempotencyKey: ulid.Make().String(),
		Data:           []byte(`{"tenant":"someone-else","plan":"free"}`),
	}))
	waitForDeliveryCount(t, env, env.Project.UID, 2, 60*time.Second)

	start := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02T15:04:05")
	end := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02T15:04:05")
	base := "/api/v1/projects/" + env.Project.UID + "/events?startDate=" + start + "&endDate=" + end

	// An exact JSON containment filter must return only the matching payload.
	body := url.QueryEscape(`{"tenant":"` + wanted + `"}`)
	matched := listEvents(t, env, base+"&body="+body)
	require.Len(t, matched.Content, 1)
	require.Equal(t, "search.match", matched.Content[0].EventType)

	// A filter that matches nothing must return nothing, not fall back to all.
	empty := listEvents(t, env, base+"&body="+url.QueryEscape(`{"tenant":"absent"}`))
	require.Empty(t, empty.Content)

	// Without a filter the same window still lists both, so the filter is what
	// narrowed the result rather than the date window.
	all := listEvents(t, env, base)
	require.Len(t, all.Content, 2)
}

func listEvents(t *testing.T, env *E2ETestEnv, path string) pagedEvents {
	t.Helper()

	data := authGet(t, env, path, env.APIKey)
	var page pagedEvents
	require.NoError(t, json.Unmarshal(data, &page))
	return page
}

// TestE2E_EventSearch_RejectsMalformedFilter checks the search rejects a body
// filter it cannot parse instead of quietly listing everything, which would
// read as a working filter to the caller.
func TestE2E_EventSearch_RejectsMalformedFilter(t *testing.T) {
	env := SetupE2E(t)

	start := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02T15:04:05")
	end := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02T15:04:05")
	path := "/api/v1/projects/" + env.Project.UID + "/events?startDate=" + start +
		"&endDate=" + end + "&body=" + url.QueryEscape(`{"tenant":`)

	_, status := rawAuthGet(t, env, path, env.APIKey)
	require.Equal(t, http.StatusBadRequest, status)
}
