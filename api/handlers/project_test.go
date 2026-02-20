package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestHandler_CreateProject_InvalidBody_Returns400(t *testing.T) {
	handler := &Handler{
		A: &types.APIOptions{
			Logger: log.FromContext(context.Background()),
			Cfg:    config.Configuration{},
		},
	}

	body := []byte(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/org-1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateProject(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response["message"], "Invalid request format")
}
