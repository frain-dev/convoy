package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/queue/drain"
	"github.com/frain-dev/convoy/util"
)

// Admission encloses the whole mutation, including database writes before an
// enqueue. Authentication and operation commands remain reachable to resume.
func (a *ApplicationHandler) queueAdmission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if a.cfg.RootPath != "" {
			path = strings.TrimPrefix(path, strings.TrimSuffix(a.cfg.RootPath, "/"))
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if path == "/ui/auth/login" || path == "/ui/auth/token/refresh" || path == "/ui/auth/logout" || strings.HasPrefix(path, "/ui/admin/queue/stores/") {
			next.ServeHTTP(w, r)
			return
		}
		response := &admissionResponse{ResponseWriter: w}
		err := a.A.QueueAdmission.Run(r.Context(), drain.Producer, func(ctx context.Context) error {
			next.ServeHTTP(response, r.WithContext(ctx))
			if response.status >= 500 {
				return drain.ErrEvidence
			}
			return nil
		})
		if err != nil && response.status == 0 {
			w.Header().Set("Retry-After", "5")
			_ = render.Render(w, r, util.NewErrorResponse("Queue admission is paused. Retry after maintenance resumes.", http.StatusServiceUnavailable))
		}
	})
}

type admissionResponse struct {
	http.ResponseWriter
	status int
}

func (w *admissionResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *admissionResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
func (w *admissionResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
