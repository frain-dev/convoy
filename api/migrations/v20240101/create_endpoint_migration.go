package v20240101

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
)

type oldCreateEndpoint struct {
	URL                string                         `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string                         `json:"secret"`
	OwnerID            string                         `json:"owner_id"`
	Description        string                         `json:"description"`
	Name               string                         `json:"name" valid:"required~please provide your endpointName"`
	SupportEmail       string                         `json:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         bool                           `json:"is_disabled"`
	SlackWebhookURL    string                         `json:"slack_webhook_url"`
	HttpTimeout        string                         `json:"http_timeout"`
	RateLimit          int                            `json:"rate_limit"`
	AdvancedSignatures *bool                          `json:"advanced_signatures"`
	RateLimitDuration  string                         `json:"rate_limit_duration"`
	Authentication     *models.EndpointAuthentication `json:"authentication"`
	AppID              string
}

type CreateEndpointRequestMigration struct{}

func (c *CreateEndpointRequestMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var payload oldCreateEndpoint
	err := json.Unmarshal(b, &payload)
	if err != nil {
		return nil, nil, err
	}

	if payload.AdvancedSignatures != nil {
		// do nothing.
		return b, h, nil
	}

	var endpoint models.CreateEndpoint
	err = json.Unmarshal(b, &endpoint)
	if err != nil {
		return nil, nil, err
	}

	// set advanced signature to the previous default.
	val := false
	endpoint.AdvancedSignatures = &val

	b, err = json.Marshal(endpoint)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}
