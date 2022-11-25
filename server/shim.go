package server

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

func (a *ApplicationHandler) RedirectToProjects(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("groupId")

	if util.IsStringEmpty(groupID) {
		groupID = r.URL.Query().Get("groupID")
	}

	if util.IsStringEmpty(groupID) {
		authUser := middleware.GetAuthUserFromContext(r.Context())

		if authUser.Credential.Type == auth.CredentialTypeAPIKey {
			groupID = authUser.Role.Group
		}
	}

	if util.IsStringEmpty(groupID) {
		_ = render.Render(w, r, util.NewErrorResponse("groupID query is missing", http.StatusBadRequest))
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
		redirectURL := fmt.Sprintf("/api/v1/projects/%s/%s?%s", groupID, forwardedPath, r.URL.RawQuery)

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
