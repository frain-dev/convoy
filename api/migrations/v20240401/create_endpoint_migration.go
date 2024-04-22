package v20240401

import (
	"encoding/json"
	"github.com/frain-dev/convoy/datastore"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
	"gopkg.in/guregu/null.v4"
)

type oldEndpointResponse struct {
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

	HttpTimeout uint64                   `json:"http_timeout" db:"http_timeout"`
	RateLimit   int                      `json:"rate_limit" db:"rate_limit"`
	Events      int64                    `json:"events,omitempty" db:"event_count"`
	Status      datastore.EndpointStatus `json:"status" db:"status"`

	RateLimitDuration uint64                            `json:"rate_limit_duration" db:"rate_limit_duration"`
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

	var old oldEndpointResponse
	err = migrateEndpoint(&endpoint, &old)
	if err != nil {
		return nil, nil, err
	}

	b, err = json.Marshal(old)
	if err != nil {
		return nil, nil, err
	}

	serverResponse.Data = b

	sb, err := json.Marshal(serverResponse)
	if err != nil {
		return nil, nil, err
	}

	return sb, h, nil
}
