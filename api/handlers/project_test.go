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

// Regression: ProjectService.UpdateProject persists config the API and
// dataplane read through the "projects:<id>" cache, so the service must be
// built on the shared (cache-invalidating) repository from h.projectRepo(),
// never on a freshly constructed uncached one that skips invalidation.
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
