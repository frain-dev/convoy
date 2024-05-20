package models

import (
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/lib/pq"
)

type CreateProject struct {
	// Project Name
	Name string `json:"name" valid:"required~please provide a valid name"`

	// Project Type, supported values are `outgoing`, `incoming`
	Type    string `json:"type" valid:"required~please provide a valid type,in(incoming|outgoing)"`
	LogoURL string `json:"logo_url" valid:"url~please provide a valid logo url,optional"`

	// Project Config
	Config *ProjectConfig `json:"config"`
}

func (cP *CreateProject) Validate() error {
	return util.Validate(cP)
}

type UpdateProject struct {
	// Project Name
	Name string `json:"name" valid:"required~please provide a valid name"`

	LogoURL string `json:"logo_url" valid:"url~please provide a valid logo url,optional"`

	// Project Config
	Config *ProjectConfig `json:"config" valid:"optional"`
}

func (uP *UpdateProject) Validate() error {
	return util.Validate(uP)
}

type ProjectConfig struct {
	// Specifies how many bytes and incoming project should read from the ingest request, and how many bytes an outgoing project should from the response of your endpoints
	// Defaults to 50KB.
	MaxIngestSize uint64 `json:"max_payload_read_size"`

	// Controls if your project will add a timestamp to it's webhook signature header to prevent a replay attack, See this blog post[https://getconvoy.io/blog/generating-stripe-like-webhook-signatures] for more]
	ReplayAttacks bool `json:"replay_attacks_prevention_enabled"`

	// Controls of the Event ID and Event Delivery ID Headers are added to the request when events are dispatched to endpoints
	AddEventIDTraceHeaders bool `json:"add_event_id_trace_headers"`

	// Controls if the project will disable and endpoint after the retry threshold for an event is reached
	DisableEndpoint bool `json:"disable_endpoint"`

	// Specify the interval in hours for which the event tokenizer runs
	SearchPolicy string `json:"search_policy" db:"search_policy"`

	// RateLimit is used to configure the projects rate limiting config values
	RateLimit *RateLimitConfiguration `json:"ratelimit"`

	// Strategy is used to configure the project's retry strategies for failing events.
	Strategy *StrategyConfiguration `json:"strategy"`

	// SSL is used to configure the project's endpoint ssl enforcement rules
	SSL *SSLConfiguration

	// Signature is used to configure the project's signature header versions
	Signature *SignatureConfiguration `json:"signature"`

	// MetaEvent is used to configure the project's meta events
	MetaEvent *MetaEventConfiguration `json:"meta_event"`

	// MultipleEndpointSubscriptions is used to configure if multiple subscriptions
	// can be created for the endpoint in a project
	MultipleEndpointSubscriptions bool `json:"multiple_endpoint_subscriptions"`
}

func (pc *ProjectConfig) Transform() *datastore.ProjectConfig {
	if pc == nil {
		return nil
	}

	return &datastore.ProjectConfig{
		MaxIngestSize:                 pc.MaxIngestSize,
		ReplayAttacks:                 pc.ReplayAttacks,
		DisableEndpoint:               pc.DisableEndpoint,
		AddEventIDTraceHeaders:        pc.AddEventIDTraceHeaders,
		MultipleEndpointSubscriptions: pc.MultipleEndpointSubscriptions,
		SSL:                           pc.SSL.transform(),
		RateLimit:                     pc.RateLimit.Transform(),
		Strategy:                      pc.Strategy.transform(),
		Signature:                     pc.Signature.transform(),
		MetaEvent:                     pc.MetaEvent.transform(),
	}
}

type SSLConfiguration struct {
	EnforceSecureEndpoints bool `json:"enforce_secure_endpoints"`
}

func (r *SSLConfiguration) transform() *datastore.SSLConfiguration {
	if r == nil {
		return nil
	}

	return &datastore.SSLConfiguration{
		EnforceSecureEndpoints: r.EnforceSecureEndpoints,
	}
}

type RateLimitConfiguration struct {
	Count    int    `json:"count"`
	Duration uint64 `json:"duration"`
}

func (rc *RateLimitConfiguration) Transform() *datastore.RateLimitConfiguration {
	if rc == nil {
		return nil
	}

	return &datastore.RateLimitConfiguration{Count: rc.Count, Duration: rc.Duration}
}

type StrategyConfiguration struct {
	Type       string `json:"type" valid:"optional~please provide a valid strategy type, in(linear|exponential)~unsupported strategy type"`
	Duration   uint64 `json:"duration" valid:"optional~please provide a valid duration in seconds,int"`
	RetryCount uint64 `json:"retry_count" valid:"optional~please provide a valid retry count,int"`
}

func (sc *StrategyConfiguration) transform() *datastore.StrategyConfiguration {
	if sc == nil {
		return nil
	}

	return &datastore.StrategyConfiguration{
		Type:       datastore.StrategyProvider(sc.Type),
		Duration:   sc.Duration,
		RetryCount: sc.RetryCount,
	}
}

type SignatureConfiguration struct {
	Header   config.SignatureHeaderProvider `json:"header,omitempty" valid:"required~please provide a valid signature header"`
	Versions []SignatureVersion             `json:"versions"`
}

func (sc *SignatureConfiguration) transform() *datastore.SignatureConfiguration {
	if sc == nil {
		return nil
	}

	s := &datastore.SignatureConfiguration{Header: sc.Header}
	for _, version := range sc.Versions {
		s.Versions = append(s.Versions, datastore.SignatureVersion{
			UID:       version.UID,
			Hash:      version.Hash,
			Encoding:  datastore.EncodingType(version.Encoding),
			CreatedAt: version.CreatedAt,
		})
	}

	return s
}

type SignatureVersion struct {
	UID       string    `json:"uid" db:"id"`
	Hash      string    `json:"hash,omitempty" valid:"required~please provide a valid hash,supported_hash~unsupported hash type"`
	Encoding  string    `json:"encoding" valid:"required~please provide a valid signature header"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type MetaEventConfiguration struct {
	IsEnabled bool           `json:"is_enabled"`
	Type      string         `json:"type" valid:"optional, in(http)~unsupported meta event type"`
	EventType pq.StringArray `json:"event_type"`
	URL       string         `json:"url"`
	Secret    string         `json:"secret"`
}

func (mc *MetaEventConfiguration) transform() *datastore.MetaEventConfiguration {
	if mc == nil {
		return nil
	}

	return &datastore.MetaEventConfiguration{
		IsEnabled: mc.IsEnabled,
		Type:      datastore.MetaEventType(mc.Type),
		EventType: mc.EventType,
		URL:       mc.URL,
		Secret:    mc.Secret,
	}
}

type ProjectResponse struct {
	*datastore.Project
}

type CreateProjectResponse struct {
	APIKey  *APIKeyResponse  `json:"api_key"`
	Project *ProjectResponse `json:"project"`
}

func NewListProjectResponse(projects []*datastore.Project) []*ProjectResponse {
	results := make([]*ProjectResponse, 0)

	for _, project := range projects {
		results = append(results, &ProjectResponse{Project: project})
	}

	return results
}
