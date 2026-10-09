package task

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// teamsSignature stands in for the sig query parameter, which is
// bearer-equivalent: it must never reach an error string or a log line.
const teamsSignature = "notasecretbutpretend"

// TestPostTeamsCard_Request asserts the exact bytes and headers that reach a
// Teams webhook. The envelope is a contract with Microsoft, so this asserts the
// literal JSON rather than field-by-field.
func TestPostTeamsCard_Request(t *testing.T) {
	var (
		gotBody        string
		gotContentType string
		gotMethod      string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotBody = string(body)
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := postTeamsCard(context.Background(), srv.Client(), srv.URL+"?sig="+teamsSignature, "endpoint disabled")
	require.NoError(t, err)

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "application/json", gotContentType)
	require.JSONEq(t, `{
		"type": "message",
		"attachments": [
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"contentUrl": null,
				"content": {
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type": "AdaptiveCard",
					"version": "1.4",
					"body": [
						{"type": "TextBlock", "text": "endpoint disabled", "wrap": true}
					]
				}
			}
		]
	}`, gotBody)
}

func TestPostTeamsCard_NonSuccessStatusDoesNotLeakSignature(t *testing.T) {
	// A provider error body commonly quotes the request URL. Nothing from the
	// body may reach the returned error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("rejected request to " + r.URL.String()))
	}))
	defer srv.Close()

	err := postTeamsCard(context.Background(), srv.Client(), srv.URL+"?sig="+teamsSignature, "endpoint disabled")

	require.ErrorIs(t, err, ErrTeamsRequestFailed)
	require.Equal(t, "failed to post teams notification: status 403", err.Error())
	require.NotContains(t, err.Error(), teamsSignature)
}

// TestPostTeamsCard_TransportErrorDoesNotLeakSignature covers the path that
// leaks by default: http.Client returns *url.Error, whose Error() quotes the full
// URL including the query string.
func TestPostTeamsCard_TransportErrorDoesNotLeakSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := srv.Client()
	srv.Close()

	err := postTeamsCard(context.Background(), client, srv.URL+"?sig="+teamsSignature, "endpoint disabled")

	require.ErrorIs(t, err, ErrTeamsRequestFailed)
	require.NotContains(t, err.Error(), teamsSignature)
	require.NotContains(t, err.Error(), srv.URL)

	// The classification has to reach the caller, not just exist: a bare
	// sentinel leaves an operator with no reason for the failure.
	require.Contains(t, err.Error(), "connection refused")
}

// TestPostTeamsCard_DrainsAndClosesBody asserts the response body is drained and
// closed rather than abandoned. Connection reuse is the observable proof: a body
// left unread pins its connection, so the second POST would open a new one.
func TestPostTeamsCard_DrainsAndClosesBody(t *testing.T) {
	conns := make(map[string]struct{})

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		// A body worth draining, well inside the read limit.
		_, _ = w.Write([]byte(strings.Repeat("a", 256)))
	}))
	srv.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns[c.RemoteAddr().String()] = struct{}{}
		}
	}
	srv.Start()
	defer srv.Close()

	for range 2 {
		require.NoError(t, postTeamsCard(context.Background(), srv.Client(), srv.URL, "endpoint disabled"))
	}

	require.Len(t, conns, 1)
}

// TestBuildTeamsAdaptiveCard_TruncationReachesTheWire guards the boundary the
// consumer depends on: the builder, not the caller, owns the size cap, so the
// full endpoint response body embedded in the shared alert text cannot push the
// serialized card past the Workflows message limit.
func TestBuildTeamsAdaptiveCard_TruncationReachesTheWire(t *testing.T) {
	const teamsMessageLimit = 28 * 1024

	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := postTeamsCard(context.Background(), srv.Client(), srv.URL, strings.Repeat("a", 64<<10))
	require.NoError(t, err)

	require.Less(t, len(gotBody), teamsMessageLimit)
	require.Contains(t, gotBody, "[truncated: alert exceeded the Microsoft Teams message size limit]")
}
