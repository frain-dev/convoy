package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (a *ApplicationHandler) RedirectToProjects(w http.ResponseWriter, r *http.Request) {
	redirectRoutes := []string{
		"/api/v1/applications",
		"/api/v1/events",
		"/api/v1/eventdeliveries",
		"/api/v1/security",
		"/api/v1/subscriptions",
		"/api/v1/sources",
	}

	for _, route := range redirectRoutes {
		if strings.HasPrefix(r.URL.Path, route) {

			groupID := r.URL.Query().Get("groupID")
			if util.IsStringEmpty(groupID) {
				_ = render.Render(w, r, util.NewErrorResponse("groupID query is missing", http.StatusBadRequest))
				return
			}

			stripped := r.URL.Path[7:] // remove the /api/v1
			redirectURL := fmt.Sprintf("/api/v1/projects/%s%s", groupID, stripped)
			r.URL.Path = redirectURL

			fwd, _ := http.NewRequest(r.Method, redirectURL, r.Body)
			fwd.Header = r.Header
			a.Router.ServeHTTP(w, fwd)
			return
		}
	}
}
