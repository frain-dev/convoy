package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestHandler_CreateProject_InvalidBody_Returns400(t *testing.T) {
	handler := &Handler{
		A: &types.APIOptions{
			Logger: log.New("convoy", log.LevelInfo),
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

// Rejecting sync_dynamic_event_ack only helps if the caller is told what
// replaced it, so the renamed-field error must survive the handler's decode
// mapping instead of collapsing into the generic message.
func TestHandler_Project_RenamedField_MessageReachesClient(t *testing.T) {
	handler := &Handler{
		A: &types.APIOptions{
			Logger: log.New("convoy", log.LevelInfo),
			Cfg:    config.Configuration{},
		},
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{
			name:   "on create",
			method: http.MethodPost,
			path:   "/api/v1/orgs/org-1/projects",
			body:   `{"name":"p","config":{"sync_dynamic_event_ack":true}}`,
			call:   handler.CreateProject,
		},
		{
			name:   "on update",
			method: http.MethodPut,
			path:   "/api/v1/projects/proj-1",
			body:   `{"config":{"sync_dynamic_event_ack":true}}`,
			call:   handler.UpdateProject,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			tc.call(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			var response map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Contains(t, response["message"], "verify_dynamic_events")
			require.NotContains(t, response["message"], "Invalid request format")
		})
	}
}

// A malformed body must still get the generic message, so surfacing the renamed
// field does not turn into echoing parser detail back to the caller.
func TestHandler_UpdateProject_InvalidBody_StaysGeneric(t *testing.T) {
	handler := &Handler{
		A: &types.APIOptions{
			Logger: log.New("convoy", log.LevelInfo),
			Cfg:    config.Configuration{},
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/proj-1", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.UpdateProject(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["message"], "Invalid request format")
}

// Regression: ProjectService.UpdateProject persists config the API and
// dataplane read through the project cache (cached.ProjectCacheKey), so the
// service must be built on the shared (cache-invalidating) repository from
// h.projectRepo(), never on a freshly constructed uncached one that skips
// invalidation.
func TestCreateProjectService_UsesSharedProjectRepo(t *testing.T) {
	ctrl := gomock.NewController(t)

	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	projectRepo := mocks.NewMockProjectRepository(ctrl)

	handler := &Handler{
		A: &types.APIOptions{
			Logger:      log.New("convoy", log.LevelInfo),
			DB:          db,
			ProjectRepo: projectRepo,
		},
	}

	svc := createProjectService(handler)

	require.Same(t, projectRepo, svc.ProjectRepo,
		"createProjectService must reuse the wired project repository")
}
