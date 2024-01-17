package v20242501

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/stretchr/testify/require"
)

func Test_Migrate(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "should_transform_advanced_signature",
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var header http.Header
			payload := &oldCreateEndpoint{
				Name: "test-endpoint",
				URL:  "https://google.com",
			}

			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}

			migration := CreateEndpointRequestMigration{}
			res, _, err := migration.Migrate(body, header)
			if err != nil {
				t.Fatal(err)
			}

			var endpoint models.CreateEndpoint
			err = json.Unmarshal(res, &endpoint)
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, *endpoint.AdvancedSignatures, false)
			require.Nil(t, err)
		})
	}
}
