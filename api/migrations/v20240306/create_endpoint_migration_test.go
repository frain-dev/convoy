package v20240306

import (
	"encoding/json"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Migrate(t *testing.T) {
	tests := []struct {
		name    string
		payload *datastore.Endpoint
	}{
		{
			name: "should_migrate_name_and_url",
			payload: &datastore.Endpoint{
				Name: "test-endpoint",
				Url:  "https://google.com",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var header http.Header = map[string][]string{"X-Convoy-version": {"2024-01-01"}}

			payloadBytes, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			serv := &util.ServerResponse{
				Status: true,
				Data:   payloadBytes,
			}

			body, err := json.Marshal(serv)
			require.NoError(t, err)

			migration := GetEndpointResponseMigration{}
			res, _, err := migration.Migrate(body, header)
			require.NoError(t, err)

			var endpoint map[string]any
			err = json.Unmarshal(res, &endpoint)
			require.NoError(t, err)

			require.Equal(t, endpoint["data"].(map[string]any)["target_url"], tc.payload.Url)
			require.Equal(t, endpoint["data"].(map[string]any)["title"], tc.payload.Name)
		})
	}
}
