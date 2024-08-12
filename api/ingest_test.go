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
		req.Header.Set("Content-Type", fmt.Sprintf("%s; boundary=$s", multipartFormDataContentType, writer.Boundary()))

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
		body := strings.NewReader("value1=key1&value2=key2")

		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", urlEncodedContentType)

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, body, payload)
	})

	t.Run("unsupported content type", func(t *testing.T) {
		jsonBody := []byte(`{"key": "value"}`)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "text/html")

		payload, err := extractPayloadFromIngestEventReq(req, 1024)
		require.NoError(t, err)
		require.Equal(t, []byte(`{"key": "value"}`), payload)
	})
}
