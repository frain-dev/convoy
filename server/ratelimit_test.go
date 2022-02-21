package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
)

func TestRateLimitByGroup(t *testing.T) {
	type test struct {
		name          string
		requestsLimit int
		windowLength  time.Duration
		groupIDs        []string
		respCodes     []int
	}
	tests := []test{
		{
			name:          "no-block",
			requestsLimit: 3,
			windowLength:  2 * time.Second,
			groupIDs:       []string {"a", "a"},
			respCodes:     []int{200, 200},
		},
		{
			name:          "block-same-group",
			requestsLimit: 2,
			windowLength:  5 * time.Second,
			groupIDs:        []string {"b", "b", "b"},
			respCodes:     []int{200, 200, 429},
		},
		{
			name: "no-block-different-group",
			requestsLimit: 1,
			windowLength: 1 * time.Second,
			groupIDs:       []string {"c", "d"},
			respCodes:     []int{200, 200},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			})
			router := rateLimitByGroupWithParams(tt.requestsLimit, tt.windowLength)(h)

			for i, code := range tt.respCodes {
				req := httptest.NewRequest("POST", "/", nil)
				req = req.Clone(context.WithValue(req.Context(), groupCtx , &convoy.Group{UID: tt.groupIDs[i]}))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				if respCode := recorder.Result().StatusCode; respCode != code {
					t.Errorf("resp.StatusCode(%v) = %v, want %v", i, respCode, code)
				}
			}
		})
	}
}