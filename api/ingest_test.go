package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_extractPayloadFromIngestEventReq(t *testing.T) {
	t.Run("application/json content type", func(t *testing.T) {
		jsonBody := []byte(`{"key": "value"}`)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", applicationJsonContentType)

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, jsonBody, payload)
	})

	t.Run("multipart/form-data content type", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("key1", "value1")
		_ = writer.WriteField("key2", "value2")
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", fmt.Sprintf("%s; boundary=%s", multipartFormDataContentType, writer.Boundary()))

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)

		var form map[string]string
		require.NoError(t, json.Unmarshal(payload, &form))

		require.Equal(t, "value1", form["key1"])
		require.Equal(t, "value2", form["key2"])
	})

	t.Run("content type not specified", func(t *testing.T) {
		jsonBody := []byte(`{"key": "value"}`)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBody))

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, jsonBody, payload)
	})

	t.Run("urlencoded content type", func(t *testing.T) {
		body := strings.NewReader("key1=value1&key2=value2")

		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", urlEncodedContentType)

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, []byte(`{"key1":"value1","key2":"value2"}`), payload)
	})

	t.Run("unsupported content type", func(t *testing.T) {
		jsonBody := []byte(`{"key": "value"}`)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "text/html")

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, jsonBody, payload)
	})
}

func Test_extractEventTypeFromLocation(t *testing.T) {
	payload := []byte(`{
		"object_kind": "push",
		"project": {"path_with_namespace": "acme/backend"},
		"user_id": 38562979,
		"confirmed": true
	}`)

	tests := []struct {
		name     string
		location string
		request  *http.Request
		want     string
		wantErr  bool
	}{
		{
			name:     "body field",
			location: "request.body.object_kind",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			want:     "push",
		},
		{
			name:     "nested body field",
			location: "request.body.project.path_with_namespace",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			want:     "acme/backend",
		},
		{
			name:     "numeric body field",
			location: "request.body.user_id",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			want:     "38562979",
		},
		{
			name:     "boolean body field",
			location: "request.body.confirmed",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			want:     "true",
		},
		{
			name:     "header field",
			location: "request.header.X-Gitlab-Event",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set("X-Gitlab-Event", "Push Hook")
				return req
			}(),
			want: "Push Hook",
		},
		{
			name:     "query field",
			location: "request.query.event_type",
			request:  httptest.NewRequest(http.MethodPost, "/?event_type=push", nil),
			want:     "push",
		},
		{
			name:     "queryparam field",
			location: "req.QueryParam.event_type",
			request:  httptest.NewRequest(http.MethodPost, "/?event_type=merge_request", nil),
			want:     "merge_request",
		},
		{
			name:     "nested header selector",
			location: "request.header.X.Event.Type",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			wantErr:  true,
		},
		{
			name:     "nested query selector",
			location: "request.query.event.type",
			request:  httptest.NewRequest(http.MethodPost, "/?event.type=push", nil),
			wantErr:  true,
		},
		{
			name:     "missing field",
			location: "request.body.event_name",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			wantErr:  true,
		},
		{
			name:     "empty header field",
			location: "request.header.X-Empty",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			wantErr:  true,
		},
		{
			name:     "invalid prefix",
			location: "payload.body.object_kind",
			request:  httptest.NewRequest(http.MethodPost, "/", nil),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractEventTypeFromLocation(tc.request, payload, tc.location)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEventTypeLocationUsesRequestMetadata(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     bool
	}{
		{name: "header location", location: "request.header.X-Gitlab-Event", want: true},
		{name: "query location", location: "request.query.event_type", want: true},
		{name: "queryparam location", location: "req.QueryParam.event_type", want: true},
		{name: "body location", location: "request.body.object_kind"},
		{name: "invalid location", location: "request.body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, eventTypeLocationUsesRequestMetadata(tc.location))
		})
	}
}

func TestSourceUsesPayloadSignature(t *testing.T) {
	tests := []struct {
		name   string
		source *datastore.Source
		want   bool
	}{
		{name: "nil source"},
		{
			name: "hmac verifier",
			source: &datastore.Source{Verifier: &datastore.VerifierConfig{
				Type: datastore.HMacVerifier,
			}},
			want: true,
		},
		{
			name:   "github provider",
			source: &datastore.Source{Provider: datastore.GithubSourceProvider},
			want:   true,
		},
		{
			name: "api key verifier",
			source: &datastore.Source{Verifier: &datastore.VerifierConfig{
				Type: datastore.APIKeyVerifier,
			}},
		},
		{
			name: "noop verifier",
			source: &datastore.Source{Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, sourceUsesPayloadSignature(tc.source))
		})
	}
}
