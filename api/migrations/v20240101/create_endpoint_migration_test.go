package v20240101

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/stretchr/testify/require"
)

var (
	truthValue = true
	falseValue = false
)

func Test_Migrate(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		payload *oldCreateEndpoint
	}{
		{
			name: "should_set_advanced_signatures_to_false_by_default",
			want: false,
			payload: &oldCreateEndpoint{
				Name: "test-endpoint",
				URL:  "https://google.com",
			},
		},
		{
			name: "should_set_advanced_signatures_to_true",
			want: true,
			payload: &oldCreateEndpoint{
				Name:               "test-endpoint",
				URL:                "https://google.com",
				AdvancedSignatures: &truthValue,
			},
		},
		{
			name: "should_set_advanced_signatures_to_false",
			want: false,
			payload: &oldCreateEndpoint{
				Name:               "test-endpoint",
				URL:                "https://google.com",
				AdvancedSignatures: &falseValue,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var header http.Header

			body, err := json.Marshal(tc.payload)
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

			require.Equal(t, *endpoint.AdvancedSignatures, tc.want)
			require.Nil(t, err)
		})
	}
}
