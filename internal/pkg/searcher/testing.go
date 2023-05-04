package searcher

import (
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/jarcoal/httpmock"
)

var successBody = []byte("event indexed successfully")
var healthCheckBody = []byte(`{}`)

// MockIndexSuccess mocks the appropriate http calls to make successfully
// mocking the `Index` call easy across the codebase
func MockIndexSuccess(t *testing.T, cfg config.SearchConfiguration) func() {
	if cfg.Type != config.TypesenseSearchProvider {
		return func() {}
	}

	url := cfg.Typesense.Host
	httpmock.Activate()

	mockHealth(url)

	httpmock.RegisterResponder(http.MethodGet, url+"/collections",
		httpmock.NewStringResponder(http.StatusOK, string(`[]`)).
			HeaderAdd(http.Header{
				"Content-Type": []string{"application/json"},
			}),
	)

	httpmock.RegisterResponder(http.MethodPost, url+"/collections",
		httpmock.NewStringResponder(http.StatusCreated, string(healthCheckBody)).
			HeaderAdd(http.Header{
				"Content-Type": []string{"application/json"},
			}),
	)

	httpmock.RegisterResponderWithQuery(http.MethodPost,
		url+"/collections/project-id-1/documents",
		"action=upsert",
		httpmock.NewStringResponder(http.StatusCreated, string(healthCheckBody)).
			HeaderAdd(http.Header{
				"Content-Type": []string{"application/json"},
			}),
	)

	return func() {
		httpmock.DeactivateAndReset()
	}
}

// MockIndexFailed mocks the appropriate http calls to make failed
// mocking the `Index` call easy across the codebase.
func MockIndexFailed(t *testing.T, cfg config.SearchConfiguration) func() {
	if cfg.Type != config.TypesenseSearchProvider {
		return func() {}
	}

	url := cfg.Typesense.Host
	httpmock.Activate()

	mockHealth(url)

	errMsg := "failed"
	httpmock.RegisterResponder(http.MethodGet, url+"/collections",
		httpmock.NewStringResponder(http.StatusBadRequest, errMsg))

	return func() {
		httpmock.DeactivateAndReset()
	}
}

func mockHealth(url string) {
	httpmock.RegisterResponder(http.MethodGet, url+"/health",
		httpmock.NewStringResponder(http.StatusOK, string(healthCheckBody)).
			HeaderAdd(http.Header{
				"Content-Type": []string{"application/json"},
			}),
	)
}
