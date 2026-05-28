package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestFilterConfigurationTransformDoesNotAliasRawMaps(t *testing.T) {
	config := &FilterConfiguration{
		EventTypes: []string{"push"},
		Filter: FS{
			Headers: datastore.M{"meta": map[string]interface{}{"event": "push"}},
			Body:    datastore.M{"payload": map[string]interface{}{"kind": "push"}},
			Query:   datastore.M{"source": map[string]interface{}{"type": "git"}},
			Path:    datastore.M{"url": map[string]interface{}{"path": "/ingest"}},
		},
	}

	transformed := config.Transform()
	err := (&transformed.Filter.Headers).Flatten()
	require.NoError(t, err)
	err = (&transformed.Filter.Body).Flatten()
	require.NoError(t, err)
	err = (&transformed.Filter.Query).Flatten()
	require.NoError(t, err)
	err = (&transformed.Filter.Path).Flatten()
	require.NoError(t, err)

	require.Equal(t, datastore.M{"meta": datastore.M{"event": "push"}}, transformed.Filter.RawHeaders)
	require.Equal(t, datastore.M{"payload": datastore.M{"kind": "push"}}, transformed.Filter.RawBody)
	require.Equal(t, datastore.M{"source": datastore.M{"type": "git"}}, transformed.Filter.RawQuery)
	require.Equal(t, datastore.M{"url": datastore.M{"path": "/ingest"}}, transformed.Filter.RawPath)
}

func TestFSTransformDoesNotAliasRawMaps(t *testing.T) {
	schema := (&FS{
		Headers: datastore.M{"meta": map[string]interface{}{"event": "push"}},
	}).Transform()

	err := (&schema.Headers).Flatten()
	require.NoError(t, err)

	require.Equal(t, datastore.M{"meta": datastore.M{"event": "push"}}, schema.RawHeaders)
}
