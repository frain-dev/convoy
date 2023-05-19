package public

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

var redirectRoutes = []string{
	"/api/v1/applications",
	"/api/v1/events",
	"/api/v1/eventdeliveries",
	"/api/v1/security",
	"/api/v1/subscriptions",
	"/api/v1/sources",
}

func (a *PublicHandler) RedirectToProjects(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectID")

	if util.IsStringEmpty(projectID) {
		projectID = r.URL.Query().Get("projectID")
	}

	if util.IsStringEmpty(projectID) {
		authUser := middleware.GetAuthUserFromContext(r.Context())

		if authUser.Credential.Type == auth.CredentialTypeAPIKey {
			projectID = authUser.Role.Project
		}
	}

	if util.IsStringEmpty(projectID) {
		_ = render.Render(w, r, util.NewErrorResponse("projectID query is missing", http.StatusBadRequest))
		return
	}

	rElems := strings.Split(r.URL.Path, "/")

	if !(cap(rElems) > 3) {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid path", http.StatusBadRequest))
		return
	}

	resourcePrefix := strings.Join(rElems[:4], "/")

	if ok := contains(redirectRoutes, resourcePrefix); ok {
		forwardedPath := strings.Join(rElems[3:], "/")
		redirectURL := fmt.Sprintf("/api/v1/projects/%s/%s?%s", projectID, forwardedPath, r.URL.RawQuery)

		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

func contains(sl []string, name string) bool {
	for _, value := range sl {
		if value == name {
			return true
		}
	}
	return false
}
