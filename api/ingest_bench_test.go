package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkExtractPayloadFromIngestEventReq(b *testing.B) {
	jsonBody := []byte(`{"event_type": "user.created", "data": {"id": 123, "name": "John Doe"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", applicationJsonContentType)
		rawPayload, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(rawPayload))

		_, err := extractPayloadFromIngestEventReq(req, rawPayload, 1024*1024)
		if err != nil {
			b.Fatal(err)
		}
	}
}
