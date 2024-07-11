package v20240101

import (
	"encoding/json"
	"net/http"

	v20240401 "github.com/frain-dev/convoy/api/migrations/v20240401"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
)

type oldUpdateEndpoint struct {
	URL                string  `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string  `json:"secret"`
	OwnerID            string  `json:"owner_id"`
	Description        string  `json:"description"`
	AdvancedSignatures *bool   `json:"advanced_signatures"`
	Name               *string `json:"name" valid:"required~please provide your endpointName"`
	SupportEmail       *string `json:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         *bool   `json:"is_disabled"`
	SlackWebhookURL    *string `json:"slack_webhook_url"`

	HttpTimeout       string                         `json:"http_timeout"`
	RateLimit         int                            `json:"rate_limit"`
	RateLimitDuration string                         `json:"rate_limit_duration"`
	Authentication    *models.EndpointAuthentication `json:"authentication"`
}

type UpdateEndpointRequestMigration struct{}

func (u *UpdateEndpointRequestMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var payload oldUpdateEndpoint
	err := json.Unmarshal(b, &payload)
	if err != nil {
		return nil, nil, err
	}

	var endpoint models.UpdateEndpoint
	err = migrateEndpoint(&payload, &endpoint, forward)
	if err != nil {
		return nil, nil, err
	}

	b, err = json.Marshal(endpoint)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}

type UpdateEndpointResponseMigration struct{}

func (u *UpdateEndpointResponseMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var serverResponse util.ServerResponse
	err := json.Unmarshal(b, &serverResponse)
	if err != nil {
		return nil, nil, err
	}

	if len(serverResponse.Data) == 0 {
		// nothing to transform.
		return b, h, nil
	}

	var endpointResp v20240401.OldEndpointResponse
	err = json.Unmarshal(serverResponse.Data, &endpointResp)
	if err != nil {
		return nil, nil, err
	}

	var oldEndpoint oldEndpoint
	err = migrateEndpoint(&endpointResp, &oldEndpoint, backward)
	if err != nil {
		return nil, nil, err
	}

	newEndpointResponse := &endpointResponse{&oldEndpoint}

	b, err = json.Marshal(newEndpointResponse)
	if err != nil {
		return nil, nil, err
	}

	serverResponse.Data = json.RawMessage(b)

	sb, err := json.Marshal(serverResponse)
	if err != nil {
		return nil, nil, err
	}

	return sb, h, nil
}
