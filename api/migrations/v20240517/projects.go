package v20240517

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
	"gopkg.in/guregu/null.v4"
)

type oldProjectResponse struct {
	UID             string                   `json:"uid" db:"id"`
	Name            string                   `json:"name" db:"name"`
	LogoURL         string                   `json:"logo_url" db:"logo_url"`
	OrganisationID  string                   `json:"organisation_id" db:"organisation_id"`
	ProjectConfigID string                   `json:"-" db:"project_configuration_id"`
	Type            ProjectType              `json:"type" db:"type"`
	Config          *datastore.ProjectConfig `json:"config" db:"config"`
	Statistics      *ProjectStatistics       `json:"statistics" db:"statistics"`

	RetainedEvents int `json:"retained_events" db:"retained_events"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type oldProjectConfig struct {
	MaxIngestSize                 uint64                  `json:"max_payload_read_size" db:"max_payload_read_size"`
	ReplayAttacks                 bool                    `json:"replay_attacks_prevention_enabled" db:"replay_attacks_prevention_enabled"`
	AddEventIDTraceHeaders        bool                    `json:"add_event_id_trace_headers"`
	DisableEndpoint               bool                    `json:"disable_endpoint" db:"disable_endpoint"`
	MultipleEndpointSubscriptions bool                    `json:"multiple_endpoint_subscriptions" db:"multiple_endpoint_subscriptions"`
	SearchPolicy                  string                  `json:"search_policy" db:"search_policy"`
	SSL                           *SSLConfiguration       `json:"ssl" db:"ssl"`
	RateLimit                     *RateLimitConfiguration `json:"ratelimit" db:"ratelimit"`
	Strategy                      *StrategyConfiguration  `json:"strategy" db:"strategy"`
	Signature                     *SignatureConfiguration `json:"signature" db:"signature"`
	MetaEvent                     *MetaEventConfiguration `json:"meta_event" db:"meta_event"`
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

	var old oldProjectResponse
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
