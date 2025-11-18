package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

func TestReactRootHandlerWithRootPath(t *testing.T) {
	testCases := []struct {
		name             string
		rootPath         string
		requestPath      string
		expectedBaseHref string
	}{
		{
			name:             "No RootPath - should use default base href",
			rootPath:         "",
			requestPath:      "/",
			expectedBaseHref: `<base href="/">`,
		},
		{
			name:             "RootPath set to /convoy - should modify base href",
			rootPath:         "/convoy",
			requestPath:      "/convoy/",
			expectedBaseHref: `<base href="/convoy/">`,
		},
		{
			name:             "RootPath set to /api/v1 - should modify base href",
			rootPath:         "/api/v1",
			requestPath:      "/api/v1/",
			expectedBaseHref: `<base href="/api/v1/">`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &ApplicationHandler{
				cfg: config.Configuration{
					RootPath: tc.rootPath,
				},
			}

			req := httptest.NewRequest(http.MethodGet, tc.requestPath, nil)
			w := httptest.NewRecorder()

			app.reactRootHandler(w, req)

			if tc.rootPath != "" {
				require.Equal(t, http.StatusOK, w.Code)
				body := w.Body.String()
				require.Contains(t, body, tc.expectedBaseHref)
			} else {
				require.NotEqual(t, http.StatusInternalServerError, w.Code)
			}
		})
	}
}

func TestReactRootHandlerStaticFiles(t *testing.T) {
	app := &ApplicationHandler{
		cfg: config.Configuration{
			RootPath: "/convoy",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/convoy/static/js/main.js", nil)
	w := httptest.NewRecorder()

	app.reactRootHandler(w, req)

	require.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestServeIndexWithRootPath(t *testing.T) {
	app := &ApplicationHandler{
		cfg: config.Configuration{
			RootPath: "/convoy",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	f := fs.FS(reactFS)
	static, err := fs.Sub(f, "ui/build")
	require.NoError(t, err)

	app.serveIndexWithRootPath(w, req, static)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()

	require.Contains(t, body, `<base href="/convoy/">`)
	require.Contains(t, body, `href="/convoy/favicon.ico"`)
}
