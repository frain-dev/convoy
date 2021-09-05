package convoy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/stretchr/testify/require"
)

var _ convoy.Plugin = (*AddHeadersPlugin)(nil)

func TestAddHeadersPlugin(t *testing.T) {

	conf := map[string]string{
		"Name": "Headers",
		"Key":  "Value",
	}

	a := &AddHeadersPlugin{config: conf}

	require.True(t, a.IsEnabled())

	w := httptest.NewRecorder()

	require.NoError(t, a.Apply(w, &http.Request{}))

	for k, v := range conf {
		require.Equal(t, v, w.Header().Get(k))
	}
}

func TestAddHeadersPlugin_IsEnabled(t *testing.T) {

	a := &AddHeadersPlugin{}

	require.False(t, a.IsEnabled())
}
