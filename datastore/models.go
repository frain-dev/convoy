package datastore

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lib/pq"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/guregu/null.v4"
)

type Pageable struct {
	Page    int `json:"page" bson:"page"`
	PerPage int `json:"per_page" bson:"per_page"`
	Sort    int `json:"sort" bson:"sort"`
}

func (p Pageable) Limit() int {
	return p.PerPage
}

func (p Pageable) Offset() int {
	return (p.Page - 1) * p.PerPage
}

type PaginationData struct {
	Total     int64 `json:"total"`
	Page      int64 `json:"page"`
	PerPage   int64 `json:"perPage"`
	Prev      int64 `json:"prev"`
	Next      int64 `json:"next"`
	TotalPage int64 `json:"totalPage"`
}

type Period int

var PeriodValues = map[string]Period{
	"daily":   Daily,
	"weekly":  Weekly,
	"monthly": Monthly,
	"yearly":  Yearly,
}

const (
	Daily Period = iota
	Weekly
	Monthly
	Yearly
)

func IsValidPeriod(period string) bool {
	_, ok := PeriodValues[period]
	return ok
}

type SearchParams struct {
	CreatedAtStart int64 `json:"created_at_start" bson:"created_at_start"`
	CreatedAtEnd   int64 `json:"created_at_end" bson:"created_at_end"`
}

type (
	StrategyProvider string
	ProjectType      string
	SourceType       string
	SourceProvider   string
	VerifierType     string
	EncodingType     string
	StorageType      string
	KeyType          string
)

type EndpointAuthenticationType string

const (
	HTTPSource     SourceType = "http"
	RestApiSource  SourceType = "rest_api"
	PubSubSource   SourceType = "pub_sub"
	DBChangeStream SourceType = "db_change_stream"
)

const (
	GithubSourceProvider  SourceProvider = "github"
	TwitterSourceProvider SourceProvider = "twitter"
	ShopifySourceProvider SourceProvider = "shopify"
)

const (
	APIKeyAuthentication EndpointAuthenticationType = "api_key"
)

func (s SourceProvider) IsValid() bool {
	switch s {
	case GithubSourceProvider, TwitterSourceProvider, ShopifySourceProvider:
		return true
	}
	return false
}

func (s SourceType) IsValid() bool {
	switch s {
	case HTTPSource, RestApiSource, PubSubSource, DBChangeStream:
		return true
	}
	return false
}

const (
	NoopVerifier      VerifierType = "noop"
	HMacVerifier      VerifierType = "hmac"
	BasicAuthVerifier VerifierType = "basic_auth"
	APIKeyVerifier    VerifierType = "api_key"
)

const (
	Base64Encoding EncodingType = "base64"
	HexEncoding    EncodingType = "hex"
)

func (e EncodingType) String() string {
	return string(e)
}

const (
	OutgoingProject ProjectType = "outgoing"
	IncomingProject ProjectType = "incoming"
)

const (
	S3     StorageType = "s3"
	OnPrem StorageType = "on_prem"
)

const (
	ProjectKey   KeyType = "project"
	AppPortalKey KeyType = "app_portal"
	CLIKey       KeyType = "cli"
	PersonalKey  KeyType = "personal_key"
)

func (k KeyType) IsValidAppKey() bool {
	switch k {
	case AppPortalKey, CLIKey:
		return true
	default:
		return false
	}
}

func (k KeyType) IsValid() bool {
	switch k {
	case AppPortalKey, CLIKey, ProjectKey, PersonalKey:
		return true
	}
	return false
}

const (
	DefaultStrategyProvider                      = LinearStrategyProvider
	LinearStrategyProvider      StrategyProvider = "linear"
	ExponentialStrategyProvider StrategyProvider = "exponential"
)

var (
	DefaultProjectConfig = ProjectConfig{
		RateLimitCount:           1000,
		RateLimitDuration:        60,
		StrategyType:             DefaultStrategyProvider,
		StrategyDuration:         100,
		StrategyRetryCount:       10,
		RetentionPolicy:          "30d",
		MaxIngestSize:            config.MaxResponseSizeKb,
		ReplayAttacks:            false,
		IsRetentionPolicyEnabled: false,
		SignatureHeader:          "X-Convoy-Signature",
		SignatureVersions: []SignatureVersion{
			{
				UID:       uuid.NewString(),
				Hash:      "SHA256",
				Encoding:  HexEncoding,
				CreatedAt: time.Now(),
			},
		},
	}

	DefaultStrategyConfig = StrategyConfiguration{
		Type:       DefaultStrategyProvider,
		Duration:   100,
		RetryCount: 10,
	}

	DefaultRateLimitConfig = RateLimitConfiguration{
		Count:    1000,
		Duration: 60,
	}

	DefaultRetryConfig = RetryConfiguration{
		Type:       LinearStrategyProvider,
		Duration:   10,
		RetryCount: 3,
	}

	DefaultAlertConfig = AlertConfiguration{
		Count:     4,
		Threshold: "1h",
	}
	DefaultStoragePolicy = StoragePolicyConfiguration{
		Type: OnPrem,
		OnPrem: &OnPremStorage{
			Path: null.NewString(convoy.DefaultOnPremDir, true),
		},
	}

	DefaultRetentionPolicy = RetentionPolicyConfiguration{
		Policy: "30d",
	}
)

func GetDefaultSignatureConfig() *SignatureConfiguration {
	return &SignatureConfiguration{
		Header: "X-Convoy-Signature",
		Versions: []SignatureVersion{
			{
				UID:       uuid.NewString(),
				Hash:      "SHA256",
				Encoding:  HexEncoding,
				CreatedAt: time.Now(),
			},
		},
	}
}

const (
	ActiveEndpointStatus   EndpointStatus = "active"
	InactiveEndpointStatus EndpointStatus = "inactive"
	PendingEndpointStatus  EndpointStatus = "pending"
)

type EndpointStatus string

type Endpoint struct {
	UID                string   `json:"uid" db:"id"`
	ProjectID          string   `json:"project_id" db:"project_id"`
	OwnerID            string   `json:"owner_id,omitempty" db:"owner_id"`
	TargetURL          string   `json:"target_url" db:"target_url"`
	Title              string   `json:"title" db:"title"`
	Secrets            []Secret `json:"secrets" db:"secrets"`
	AdvancedSignatures bool     `json:"advanced_signatures" db:"advanced_signatures"`
	Description        string   `json:"description" db:"description"`
	SlackWebhookURL    string   `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	SupportEmail       string   `json:"support_email,omitempty" db:"support_email"`
	AppID              string   `json:"-" db:"app_id"` // Deprecated but necessary for backward compatibility

	HttpTimeout string         `json:"http_timeout" db:"http_timeout"`
	RateLimit   int            `json:"rate_limit" db:"rate_limit"`
	Events      int64          `json:"events,omitempty" db:"-"`
	Status      EndpointStatus `json:"status" db:"status"`

	RateLimitDuration string                  `json:"rate_limit_duration" db:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication" db:"authentication"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func (e *Endpoint) GetAuthConfig() EndpointAuthentication {
	if e.Authentication != nil {
		if e.Authentication.ApiKey != nil {
			return *e.Authentication
		}
	}

	return EndpointAuthentication{ApiKey: &ApiKey{}}
}

func (e *Endpoint) GetActiveSecretIndex() (int, error) {
	for idx, secret := range e.Secrets {
		if secret.ExpiresAt.IsZero() {
			return idx, nil
		}
	}
	return 0, ErrNoActiveSecret
}

type Secret struct {
	UID   string `json:"uid" db:"id"`
	Value string `json:"value" db:"value"`

	ExpiresAt null.Time `json:"expires_at,omitempty" db:"expires_at,omitempty" swaggertype:"string"`
	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type EndpointAuthentication struct {
	Type   EndpointAuthenticationType `json:"type,omitempty" db:"type" valid:"optional,in(api_key)~unsupported authentication type"`
	ApiKey *ApiKey                    `json:"api_key" db:"api_key"`
}

var (
	ErrOrgNotFound       = errors.New("organisation not found")
	ErrDeviceNotFound    = errors.New("device not found")
	ErrOrgInviteNotFound = errors.New("organisation invite not found")
	ErrOrgMemberNotFound = errors.New("organisation member not found")
)

type Project struct {
	UID             string             `json:"uid" db:"id"`
	Name            string             `json:"name" db:"name"`
	LogoURL         string             `json:"logo_url" db:"logo_url"`
	OrganisationID  string             `json:"organisation_id" db:"organisation_id"`
	ProjectConfigID string             `json:"-" db:"project_configuration_id"`
	Type            ProjectType        `json:"type" db:"type"`
	Config          *ProjectConfig     `json:"config" db:"config"`
	Statistics      *ProjectStatistics `json:"statistics" db:"-"`

	RetainedEvents int `json:"retained_events" db:"retained_events"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type ProjectMetadata struct {
	RetainedEvents int `json:"retained_events" bson:"retained_events"`
}

type ProjectConfig struct {
	RetentionPolicy          string                         `json:"retention_policy" db:"retention_policy" valid:"required~please provide a valid retention policy"`
	RateLimitCount           int                            `json:"ratelimit.count" db:"ratelimit_count"`
	RateLimitDuration        int                            `json:"ratelimit.duration" db:"ratelimit_duration"`
	StrategyType             StrategyProvider               `json:"strategy_type" db:"strategy_type" valid:"optional~please provide a valid strategy type, in(linear|exponential)~unsupported strategy type"`
	StrategyDuration         uint64                         `json:"strategy_duration" db:"strategy_duration" valid:"optional~please provide a valid duration in seconds,int"`
	StrategyRetryCount       uint64                         `json:"strategy_retry_count" db:"strategy_retry_count" valid:"optional~please provide a valid retry count,int"`
	SignatureHeader          config.SignatureHeaderProvider `json:"signature_header" db:"signature_header" valid:"required~please provide a valid signature header"`
	SignatureVersions        []SignatureVersion             `json:"versions" db:"-"`
	Versions                 []byte                         `json:"-" db:"signature_versions"`
	MaxIngestSize            uint64                         `json:"max_payload_read_size" db:"max_payload_read_size"`
	ReplayAttacks            bool                           `json:"replay_attacks_prevention_enabled" db:"replay_attacks_prevention_enabled"`
	IsRetentionPolicyEnabled bool                           `json:"retention_policy_enabled" db:"retention_policy_enabled"`

	// old fields
	RateLimit *RateLimitConfiguration `json:"ratelimit"`
	Strategy  *StrategyConfiguration  `json:"strategy"`
	Signature *SignatureConfiguration `json:"signature"`
	// RetentionPolicy *RetentionPolicyConfiguration `json:"policy" bson:"retention_policy"`
}

type RateLimitConfiguration struct {
	Count    int    `json:"count" bson:"count"`
	Duration uint64 `json:"duration" bson:"duration"`
}

type StrategyConfiguration struct {
	Type       StrategyProvider `json:"type" valid:"optional~please provide a valid strategy type, in(linear|exponential)~unsupported strategy type"`
	Duration   uint64           `json:"duration" valid:"optional~please provide a valid duration in seconds,int"`
	RetryCount uint64           `json:"retry_count" valid:"optional~please provide a valid retry count,int"`
}

type SignatureConfiguration struct {
	Header   config.SignatureHeaderProvider `json:"header,omitempty" valid:"required~please provide a valid signature header"`
	Versions []SignatureVersion             `json:"versions" bson:"versions"`

	Hash string `json:"-" bson:"hash"`
}

type SignatureVersion struct {
	UID       string       `json:"uid" db:"id"`
	Hash      string       `json:"hash,omitempty" valid:"required~please provide a valid hash,supported_hash~unsupported hash type"`
	Encoding  EncodingType `json:"encoding" db:"encoding" valid:"required~please provide a valid signature header"`
	CreatedAt time.Time    `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
}

type RetentionPolicyConfiguration struct {
	Policy string `json:"policy" valid:"required~please provide a valid retention policy"`
}

type ProjectStatistics struct {
	ProjectID    string `json:"-" db:"-"`
	MessagesSent int64  `json:"messages_sent" db:"messages_sent"`
	TotalApps    int64  `json:"total_endpoints" db:"total_endpoints"`
}

type ProjectFilter struct {
	OrgID string `json:"org_id" bson:"org_id"`
}

type EventFilter struct {
	ProjectID      string `json:"project_id" bson:"project_id"`
	CreatedAtStart int64  `json:"created_at_start" bson:"created_at_start"`
	CreatedAtEnd   int64  `json:"created_at_end" bson:"created_at_end"`
}

type EventDeliveryFilter struct {
	ProjectID      string `json:"project_id" bson:"project_id"`
	CreatedAtStart int64  `json:"created_at_start" bson:"created_at_start"`
	CreatedAtEnd   int64  `json:"created_at_end" bson:"created_at_end"`
}

// func (g *ProjectFilter) WithNamesTrimmed() *ProjectFilter {
// 	f := ProjectFilter{OrgID: g.OrgID, Names: []string{}}

// 	for _, s := range g.Names {
// 		s = strings.TrimSpace(s)
// 		if len(s) == 0 {
// 			continue
// 		}
// 		f.Names = append(f.Names, s)
// 	}

// 	return &f
// }

// func (g *ProjectFilter) ToGenericMap() map[string]interface{} {
// 	m := map[string]interface{}{"name": g.Names}
// 	return m
// }

func (o *Project) IsDeleted() bool { return o.DeletedAt.Valid }

func (o *Project) IsOwner(e *Endpoint) bool { return o.UID == e.ProjectID }

var (
	ErrUserNotFound                  = errors.New("user not found")
	ErrSourceNotFound                = errors.New("source not found")
	ErrEventNotFound                 = errors.New("event not found")
	ErrProjectNotFound               = errors.New("project not found")
	ErrAPIKeyNotFound                = errors.New("api key not found")
	ErrEndpointNotFound              = errors.New("endpoint not found")
	ErrSubscriptionNotFound          = errors.New("subscription not found")
	ErrEventDeliveryNotFound         = errors.New("event delivery not found")
	ErrEventDeliveryAttemptNotFound  = errors.New("event delivery attempt not found")
	ErrPortalLinkNotFound            = errors.New("portal link not found")
	ErrDuplicateEndpointName         = errors.New("an endpoint with this name exists")
	ErrNotAuthorisedToAccessDocument = errors.New("your credentials cannot access or modify this resource")
	ErrConfigNotFound                = errors.New("config not found")
	ErrDuplicateProjectName          = errors.New("a project with this name already exists")
	ErrDuplicateEmail                = errors.New("a user with this email already exists")
	ErrNoActiveSecret                = errors.New("no active secret found")
)

type AppMetadata struct {
	UID          string `json:"uid" bson:"uid"`
	Title        string `json:"title" bson:"title"`
	ProjectID    string `json:"project_id" bson:"project_id"`
	SupportEmail string `json:"support_email" bson:"support_email"`
}

// EventType is used to identify a specific event.
// This could be "user.new"
// This will be used for data indexing
// Makes it easy to filter by a list of events
type EventType string

// Event defines a payload to be sent to an application
type Event struct {
	UID              string    `json:"uid" db:"id"`
	EventType        EventType `json:"event_type" db:"event_type"`
	MatchedEndpoints int       `json:"matched_endpoints" db:"matched_enpoints"` // TODO(all) remove this field

	SourceID         string                `json:"source_id,omitempty" db:"source_id"`
	AppID            string                `json:"app_id,omitempty" db:"app_id"` // Deprecated
	ProjectID        string                `json:"project_id,omitempty" db:"project_id"`
	Endpoints        []string              `json:"endpoints" db:"endpoints"`
	Headers          httpheader.HTTPHeader `json:"headers" db:"headers"`
	EndpointMetadata []*Endpoint           `json:"endpoint_metadata,omitempty" db:"endpoint_metadata"`
	Source           *Source               `json:"source_metadata,omitempty" db:"source_metadata"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data,omitempty" db:"data"`
	Raw  string          `json:"raw,omitempty" db:"raw"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func (e *Event) GetRawHeaders() map[string]interface{} {
	h := map[string]interface{}{}
	for k, v := range e.Headers {
		h[k] = v
	}
	return h
}

func (e *Event) GetRawHeadersJSON() ([]byte, error) {
	h := map[string]interface{}{}
	for k, v := range e.Headers {
		h[k] = v
	}

	return json.Marshal(h)
}

type (
	SubscriptionType    string
	EventDeliveryStatus string
	HttpHeader          map[string]string
)

func (h HttpHeader) SetHeadersInRequest(r *http.Request) {
	for k, v := range h {
		r.Header.Set(k, v)
	}
}

const (
	// ScheduledEventStatus : when an Event has been scheduled for delivery
	ScheduledEventStatus  EventDeliveryStatus = "Scheduled"
	ProcessingEventStatus EventDeliveryStatus = "Processing"
	DiscardedEventStatus  EventDeliveryStatus = "Discarded"
	FailureEventStatus    EventDeliveryStatus = "Failure"
	SuccessEventStatus    EventDeliveryStatus = "Success"
	RetryEventStatus      EventDeliveryStatus = "Retry"
)

func (e EventDeliveryStatus) IsValid() bool {
	switch e {
	case ScheduledEventStatus,
		ProcessingEventStatus,
		DiscardedEventStatus,
		FailureEventStatus,
		SuccessEventStatus,
		RetryEventStatus:
		return true
	default:
		return false
	}
}

const (
	SubscriptionTypeCLI SubscriptionType = "cli"
	SubscriptionTypeAPI SubscriptionType = "api"
)

type Metadata struct {
	// Data to be sent to endpoint.
	Data     json.RawMessage  `json:"data" bson:"data"`
	Raw      string           `json:"raw" bson:"raw"`
	Strategy StrategyProvider `json:"strategy" bson:"strategy"`

	NextSendTime time.Time `json:"next_send_time" bson:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this Event to
	// an application
	NumTrials uint64 `json:"num_trials" bson:"num_trials"`

	IntervalSeconds uint64 `json:"interval_seconds" bson:"interval_seconds"`

	RetryLimit uint64 `json:"retry_limit" bson:"retry_limit"`
}

func (em Metadata) Value() (driver.Value, error) {
	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(em); err != nil {
		return driver.Value(""), err
	}

	return driver.Value(b.String()), nil
}

type EventIntervalData struct {
	Interval int64  `json:"index" db:"index"`
	Time     string `json:"date" db:"total_time"`
}

type EventInterval struct {
	Data  EventIntervalData `json:"data" db:"data"`
	Count uint64            `json:"count" db:"count"`
}

type DeliveryAttempt struct {
	ID         primitive.ObjectID `json:"-" bson:"_id"`
	UID        string             `json:"uid" bson:"uid"`
	MsgID      string             `json:"msg_id" bson:"msg_id"`
	URL        string             `json:"url" bson:"url"`
	Method     string             `json:"method" bson:"method"`
	EndpointID string             `json:"endpoint_id" bson:"endpoint_id"`
	APIVersion string             `json:"api_version" bson:"api_version"`

	IPAddress        string     `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	RequestHeader    HttpHeader `json:"request_http_header,omitempty" bson:"request_http_header,omitempty"`
	ResponseHeader   HttpHeader `json:"response_http_header,omitempty" bson:"response_http_header,omitempty"`
	HttpResponseCode string     `json:"http_status,omitempty" bson:"http_status,omitempty"`
	ResponseData     string     `json:"response_data,omitempty" bson:"response_data,omitempty"`
	Error            string     `json:"error,omitempty" bson:"error,omitempty"`
	Status           bool       `json:"status,omitempty" bson:"status,omitempty"`

	CreatedAt primitive.DateTime  `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime  `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt *primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at" swaggertype:"string"`
}

// EventDelivery defines a payload to be sent to an endpoint
type EventDelivery struct {
	UID            string                `json:"uid" db:"id"`
	ProjectID      string                `json:"project_id,omitempty" db:"project_id"`
	EventID        string                `json:"event_id,omitempty" db:"event_id"`
	EndpointID     string                `json:"endpoint_id,omitempty" db:"endpoint_id"`
	DeviceID       string                `json:"device_id" db:"device_id"`
	SubscriptionID string                `json:"subscription_id,omitempty" db:"subscription_id"`
	Headers        httpheader.HTTPHeader `json:"headers" db:"headers"`

	Endpoint *Endpoint `json:"endpoint_metadata,omitempty" db:"endpoint_metadata"`
	Event    *Event    `json:"event_metadata,omitempty" db:"event_metadata"`

	DeliveryAttempts []DeliveryAttempt   `json:"-" db:"attempts"`
	Status           EventDeliveryStatus `json:"status" db:"status"`
	Metadata         *Metadata           `json:"metadata" db:"metadata"`
	CLIMetadata      *CLIMetadata        `json:"cli_metadata" db:"cli_metadata"`
	Description      string              `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time           `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt        time.Time           `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt        null.Time           `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type CLIMetadata struct {
	EventType string `json:"event_type" bson:"event_type"`
	HostName  string `json:"host_name,omitempty" bson:"-"`
}

type APIKey struct {
	// ID        primitive.ObjectID  `json:"-" bson:"_id"`
	UID       string    `json:"uid" db:"id"`
	MaskID    string    `json:"mask_id,omitempty" db:"mask_id"`
	Name      string    `json:"name" db:"name"`
	Role      auth.Role `json:"role" db:"role"`
	Hash      string    `json:"hash,omitempty" db:"hash"`
	Salt      string    `json:"salt,omitempty" db:"salt"`
	Type      KeyType   `json:"key_type" db:"key_type"`
	UserID    string    `json:"user_id" db:"user_id"`
	ExpiresAt time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

type Subscription struct {
	UID        string           `json:"uid" db:"id"`
	Name       string           `json:"name" db:"name"`
	Type       SubscriptionType `json:"type" db:"type"`
	ProjectID  string           `json:"-" db:"project_id"`
	SourceID   string           `json:"-" db:"source_id"`
	EndpointID string           `json:"-" db:"endpoint_id"`
	DeviceID   string           `json:"device_id" db:"device_id"`

	Source   *Source   `json:"source_metadata" db:"source_metadata"`
	Endpoint *Endpoint `json:"endpoint_metadata" db:"endpoint_metadata"`

	// subscription config
	AlertConfig     *AlertConfiguration     `json:"alert_config,omitempty" db:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration     `json:"retry_config,omitempty" db:"retry_config,omitempty"`
	FilterConfig    *FilterConfiguration    `json:"filter_config,omitempty" db:"filter_config,omitempty"`
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty" db:"rate_limit_config,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

// For DB access
func (s *Subscription) GetAlertConfig() AlertConfiguration {
	if s.AlertConfig != nil {
		return *s.AlertConfig
	}
	return AlertConfiguration{}
}

func (s *Subscription) GetRetryConfig() RetryConfiguration {
	if s.RetryConfig != nil {
		return *s.RetryConfig
	}
	return RetryConfiguration{}
}

func (s *Subscription) GetFilterConfig() FilterConfiguration {
	if s.FilterConfig != nil {
		return *s.FilterConfig
	}
	return FilterConfiguration{}
}

func (s *Subscription) GetRateLimitConfig() RateLimitConfiguration {
	if s.RateLimitConfig != nil {
		return *s.RateLimitConfig
	}
	return RateLimitConfiguration{}
}

type Source struct {
	UID            string          `json:"uid" db:"id"`
	ProjectID      string          `json:"project_id" db:"project_id"`
	MaskID         string          `json:"mask_id" db:"mask_id"`
	Name           string          `json:"name" db:"name"`
	Type           SourceType      `json:"type" db:"type"`
	Provider       SourceProvider  `json:"provider" db:"provider"`
	IsDisabled     bool            `json:"is_disabled" db:"is_disabled"`
	VerifierID     string          `json:"-" db:"source_verifier_id"`
	Verifier       *VerifierConfig `json:"verifier" db:"verifier"`
	ProviderConfig *ProviderConfig `json:"provider_config" db:"provider_config"`
	ForwardHeaders pq.StringArray  `json:"forward_headers" db:"forward_headers"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type User struct {
	UID                        string    `json:"uid" db:"id"`
	FirstName                  string    `json:"first_name" db:"first_name"`
	LastName                   string    `json:"last_name" db:"last_name"`
	Email                      string    `json:"email" db:"email"`
	EmailVerified              bool      `json:"email_verified" db:"email_verified"`
	Password                   string    `json:"-" db:"password"`
	ResetPasswordToken         string    `json:"-" db:"reset_password_token"`
	EmailVerificationToken     string    `json:"-" db:"email_verification_token"`
	CreatedAt                  time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt                  time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt                  null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
	ResetPasswordExpiresAt     time.Time `json:"reset_password_expires_at,omitempty" db:"reset_password_expires_at,omitempty" swaggertype:"string"`
	EmailVerificationExpiresAt time.Time `json:"-" db:"email_verification_expires_at,omitempty" swaggertype:"string"`
}

type RetryConfiguration struct {
	Type       StrategyProvider `json:"type,omitempty" bson:"type,omitempty" valid:"supported_retry_strategy~please provide a valid retry strategy type"`
	Duration   uint64           `json:"duration,omitempty" bson:"duration,omitempty" valid:"duration~please provide a valid time duration"`
	RetryCount uint64           `json:"retry_count" bson:"retry_count" valid:"int~please provide a valid retry count"`
}

type AlertConfiguration struct {
	Count     int    `json:"count" bson:"count,omitempty"`
	Threshold string `json:"threshold" bson:"threshold,omitempty" valid:"duration~please provide a valid time duration"`
}

type FilterConfiguration struct {
	EventTypes pq.StringArray `json:"event_types" bson:"event_types,omitempty"`
	Filter     FilterSchema   `json:"filter" bson:"filter"`
}

type FilterSchema struct {
	Headers map[string]interface{} `json:"headers" bson:"headers"`
	Body    map[string]interface{} `json:"body" bson:"body"`
}

type ProviderConfig struct {
	Twitter *TwitterProviderConfig `json:"twitter" bson:"twitter"`
}

type TwitterProviderConfig struct {
	CrcVerifiedAt primitive.DateTime `json:"crc_verified_at" bson:"crc_verified_at"`
}

type VerifierConfig struct {
	Type      VerifierType `json:"type,omitempty" db:"type" valid:"supported_verifier~please provide a valid verifier type,required"`
	HMac      *HMac        `json:"hmac" db:"hmac"`
	BasicAuth *BasicAuth   `json:"basic_auth" db:"basic_auth"`
	ApiKey    *ApiKey      `json:"api_key" db:"api_key"`
}

type HMac struct {
	Header   string       `json:"header" db:"header" valid:"required"`
	Hash     string       `json:"hash" db:"hash" valid:"supported_hash,required"`
	Secret   string       `json:"secret" db:"secret" valid:"required"`
	Encoding EncodingType `json:"encoding" db:"encoding" valid:"supported_encoding~please provide a valid encoding type,required"`
}

type BasicAuth struct {
	UserName string `json:"username" db:"username" valid:"required" `
	Password string `json:"password" db:"password" valid:"required"`
}

type ApiKey struct {
	HeaderValue string `json:"header_value" db:"header_value" valid:"required"`
	HeaderName  string `json:"header_name" db:"header_name" valid:"required"`
}

type Organisation struct {
	UID            string      `json:"id" db:"id"`
	OwnerID        string      `json:"" db:"owner_id"`
	Name           string      `json:"name" db:"name"`
	CustomDomain   null.String `json:"custom_domain" db:"custom_domain"`
	AssignedDomain null.String `json:"assigned_domain" db:"assigned_domain"`
	CreatedAt      time.Time   `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt      time.Time   `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt      null.Time   `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type Configuration struct {
	UID                string                      `json:"uid" db:"id"`
	IsAnalyticsEnabled bool                        `json:"is_analytics_enabled" db:"is_analytics_enabled"`
	IsSignupEnabled    bool                        `json:"is_signup_enabled" db:"is_signup_enabled"`
	StoragePolicy      *StoragePolicyConfiguration `json:"storage_policy" db:"storage_policy"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type StoragePolicyConfiguration struct {
	Type   StorageType    `json:"type,omitempty" db:"type" valid:"supported_storage~please provide a valid storage type,required"`
	S3     *S3Storage     `json:"s3" db:"s3"`
	OnPrem *OnPremStorage `json:"on_prem" db:"on_prem"`
}

type S3Storage struct {
	Bucket       null.String `json:"bucket" db:"bucket" valid:"required~please provide a bucket name"`
	AccessKey    null.String `json:"access_key,omitempty" db:"access_key" valid:"required~please provide an access key"`
	SecretKey    null.String `json:"secret_key,omitempty" db:"secret_key" valid:"required~please provide a secret key"`
	Region       null.String `json:"region,omitempty" db:"region"`
	SessionToken null.String `json:"session_token" db:"session_token"`
	Endpoint     null.String `json:"endpoint,omitempty" db:"endpoint"`
}

type OnPremStorage struct {
	Path null.String `json:"path" db:"path"`
}

type OrganisationMember struct {
	UID            string       `json:"uid" db:"id"`
	OrganisationID string       `json:"organisation_id" db:"organisation_id"`
	UserID         string       `json:"user_id" db:"user_id"`
	Role           auth.Role    `json:"role" db:"role"`
	UserMetadata   UserMetadata `json:"user_metadata" db:"user_metadata"`
	CreatedAt      time.Time    `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt      time.Time    `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt      null.Time    `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type Device struct {
	UID        string       `json:"uid" db:"id"`
	ProjectID  string       `json:"project_id,omitempty" db:"project_id"`
	EndpointID string       `json:"endpoint_id,omitempty" db:"endpoint_id"`
	HostName   string       `json:"host_name,omitempty" db:"host_name"`
	Status     DeviceStatus `json:"status,omitempty" db:"status"`
	LastSeenAt time.Time    `json:"last_seen_at,omitempty" db:"last_seen_at,omitempty" swaggertype:"string"`
	CreatedAt  time.Time    `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt  time.Time    `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt  null.Time    `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type DeviceStatus string

const (
	DeviceStatusOffline  DeviceStatus = "offline"
	DeviceStatusOnline   DeviceStatus = "online"
	DeviceStatusDisabled DeviceStatus = "disabled"
)

type UserMetadata struct {
	UserID    string `json:"-" db:"user_id"`
	FirstName string `json:"first_name" db:"first_name"`
	LastName  string `json:"last_name" db:"last_name"`
	Email     string `json:"email" db:"email"`
}

type InviteStatus string

const (
	InviteStatusAccepted  InviteStatus = "accepted"
	InviteStatusDeclined  InviteStatus = "declined"
	InviteStatusPending   InviteStatus = "pending"
	InviteStatusCancelled InviteStatus = "cancelled"
)

func (i InviteStatus) String() string {
	return string(i)
}

type OrganisationInvite struct {
	UID            string       `json:"uid" db:"id"`
	OrganisationID string       `json:"organisation_id" db:"organisation_id"`
	InviteeEmail   string       `json:"invitee_email" db:"invitee_email"`
	Token          string       `json:"token" db:"token"`
	Role           auth.Role    `json:"role" db:"role"`
	Status         InviteStatus `json:"status" db:"status"`
	ExpiresAt      time.Time    `json:"-" db:"expires_at"`
	CreatedAt      time.Time    `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt      time.Time    `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt      null.Time    `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type PortalLink struct {
	UID               string     `json:"uid" db:"id"`
	Name              string     `json:"name" db:"name"`
	ProjectID         string     `json:"project_id" db:"project_id"`
	Token             string     `json:"-" db:"token"`
	Endpoints         []string   `json:"endpoints" db:"endpoints"`
	EndpointsMetadata []Endpoint `json:"endpoints_metadata" db:"endpoints_metadata"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at,omitempty" swaggertype:"string"`
}

// Deprecated
type Application struct {
	ID              primitive.ObjectID `json:"-" db:"_id"`
	UID             string             `json:"uid" db:"uid"`
	ProjectID       string             `json:"project_id" db:"project_id"`
	Title           string             `json:"name" db:"title"`
	SupportEmail    string             `json:"support_email,omitempty" db:"support_email"`
	SlackWebhookURL string             `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	IsDisabled      bool               `json:"is_disabled,omitempty" db:"is_disabled"`

	Endpoints []DeprecatedEndpoint `json:"endpoints,omitempty" db:"endpoints"`
	CreatedAt time.Time            `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time            `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time            `json:"deleted_at,omitempty" db:"deleted_at,omitempty" swaggertype:"string"`

	Events int64 `json:"events,omitempty" db:"-"`
}

// Deprecated
type DeprecatedEndpoint struct {
	UID                string   `json:"uid" db:"uid"`
	TargetURL          string   `json:"target_url" db:"target_url"`
	Description        string   `json:"description" db:"description"`
	Secret             string   `json:"-" db:"secret"`
	Secrets            []Secret `json:"secrets" db:"secrets"`
	AdvancedSignatures bool     `json:"advanced_signatures" db:"advanced_signatures"`

	HttpTimeout       string                  `json:"http_timeout" db:"http_timeout"`
	RateLimit         int                     `json:"rate_limit" db:"rate_limit"`
	RateLimitDuration string                  `json:"rate_limit_duration" db:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication" db:"authentication,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at,omitempty" swaggertype:"string"`
}

type Password struct {
	Plaintext string
	Hash      []byte
}

func (p *Password) GenerateHash() error {
	hash, err := bcrypt.GenerateFromPassword([]byte(p.Plaintext), 12)
	if err != nil {
		return err
	}

	p.Hash = hash
	return nil
}

func (p *Password) Matches() (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.Hash, []byte(p.Plaintext))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, err
}

type SubscriptionFilter struct {
	ID        primitive.ObjectID     `json:"-" bson:"_id"`
	UID       string                 `json:"uid" bson:"uid"`
	Filter    map[string]interface{} `json:"filter" bson:"filter"`
	DeletedAt *primitive.DateTime    `json:"deleted_at,omitempty" bson:"deleted_at"`
}
