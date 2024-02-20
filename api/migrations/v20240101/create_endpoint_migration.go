package v20240101

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/guregu/null/v5"
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

	var endpoint models.CreateEndpoint

	err = migrateEndpoint(&payload, &endpoint, forward)
	if err != nil {
		return nil, nil, err
	}

	if payload.AdvancedSignatures == nil {
		// set advanced signature to the previous default.
		val := false
		endpoint.AdvancedSignatures = &val
	}

	b, err = json.Marshal(endpoint)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}

type endpointResponse struct {
	*oldEndpoint
}

type oldEndpoint struct {
	UID                string            `json:"uid" db:"id"`
	ProjectID          string            `json:"project_id" db:"project_id"`
	OwnerID            string            `json:"owner_id,omitempty" db:"owner_id"`
	TargetURL          string            `json:"target_url" db:"target_url"`
	Title              string            `json:"title" db:"title"`
	Secrets            datastore.Secrets `json:"secrets" db:"secrets"`
	AdvancedSignatures bool              `json:"advanced_signatures" db:"advanced_signatures"`
	Description        string            `json:"description" db:"description"`
	SlackWebhookURL    string            `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	SupportEmail       string            `json:"support_email,omitempty" db:"support_email"`
	AppID              string            `json:"-" db:"app_id"` // Deprecated but necessary for backward compatibility

	HttpTimeout string                   `json:"http_timeout" db:"http_timeout"`
	RateLimit   int                      `json:"rate_limit" db:"rate_limit"`
	Events      int64                    `json:"events,omitempty" db:"event_count"`
	Status      datastore.EndpointStatus `json:"status" db:"status"`

	RateLimitDuration string                            `json:"rate_limit_duration" db:"rate_limit_duration"`
	Authentication    *datastore.EndpointAuthentication `json:"authentication" db:"authentication"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type CreateEndpointResponseMigration struct{}

func (c *CreateEndpointResponseMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var serverResponse util.ServerResponse
	err := json.Unmarshal(b, &serverResponse)
	if err != nil {
		return nil, nil, err
	}

	if len(serverResponse.Data) == 0 {
		// nothing to transform.
		return b, h, nil
	}

	var endpointResp *models.EndpointResponse
	err = json.Unmarshal(serverResponse.Data, &endpointResp)
	if err != nil {
		return nil, nil, err
	}

	endpoint := endpointResp.Endpoint

	var oldEndpoint oldEndpoint
	err = migrateEndpoint(&endpoint, &oldEndpoint, backward)
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
