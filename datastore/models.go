package datastore

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/guregu/null.v4"
)

type Pageable struct {
	PerPage    int           `json:"per_page"`
	Direction  PageDirection `json:"direction"`
	PrevCursor string        `json:"prev_page_cursor"`
	NextCursor string        `json:"next_page_cursor"`
}

type PageDirection string

const Next PageDirection = "next"
const Prev PageDirection = "prev"

func (p Pageable) Cursor() string {
	if p.Direction == Next {
		return p.NextCursor
	}

	return p.PrevCursor
}

func (p Pageable) Limit() int {
	return p.PerPage + 1
}

type PaginationData struct {
	PrevRowCount    PrevRowCount `json:"-"`
	PerPage         int64        `json:"per_page"`
	HasNextPage     bool         `json:"has_next_page"`
	HasPreviousPage bool         `json:"has_prev_page"`
	PrevPageCursor  string       `json:"prev_page_cursor"`
	NextPageCursor  string       `json:"next_page_cursor"`
}

type PrevRowCount struct {
	Count int
}

func (p *PaginationData) Build(pageable Pageable, items []string) *PaginationData {
	p.PerPage = int64(pageable.PerPage)

	var s, e string

	if len(items) > 0 {
		s = items[0]
	}

	if len(items) > 1 {
		e = items[len(items)-1]
	}

	p.PrevPageCursor = s
	p.NextPageCursor = e

	// there's an extra item. We use it to find out if there is more data to be loaded
	if len(items) > pageable.PerPage {
		p.HasNextPage = true
	}

	if p.PrevRowCount.Count > 0 {
		p.HasPreviousPage = true
	}

	return p
}

type Period int

var PeriodValues = map[string]Period{
	"daily":   Daily,
	"weekly":  Weekly,
	"monthly": Monthly,
	"yearly":  Yearly,
}

var DefaultCursor = fmt.Sprintf("%d", math.MaxInt)

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
	PubSubType       string
	PubSubHandler    func(context.Context, *Source, string) error
	MetaEventType    string
	HookEventType    string
)

type EndpointAuthenticationType string

const (
	HTTPSource     SourceType = "http"
	RestApiSource  SourceType = "rest_api"
	PubSubSource   SourceType = "pub_sub"
	DBChangeStream SourceType = "db_change_stream"
)

const (
	HTTPMetaEvent   MetaEventType = "http"
	PubSubMetaEvent MetaEventType = "pub_sub"
)

const (
	EndpointCreated      HookEventType = "endpoint.created"
	EndpointUpdated      HookEventType = "endpoint.updated"
	EndpointDeleted      HookEventType = "endpoint.deleted"
	EventDeliveryUpdated HookEventType = "eventdelivery.updated"
	EventDeliverySuccess HookEventType = "eventdelivery.success"
	EventDeliveryFailed  HookEventType = "eventdelivery.failed"
)

const (
	GithubSourceProvider  SourceProvider = "github"
	TwitterSourceProvider SourceProvider = "twitter"
	ShopifySourceProvider SourceProvider = "shopify"
)

const (
	APIKeyAuthentication EndpointAuthenticationType = "api_key"
)

const (
	SqsPubSub    PubSubType = "sqs"
	GooglePubSub PubSubType = "google"
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
		RetentionPolicy:          &DefaultRetentionPolicy,
		MaxIngestSize:            config.MaxResponseSize,
		ReplayAttacks:            false,
		IsRetentionPolicyEnabled: false,
		DisableEndpoint:          false,
		RateLimit:                &DefaultRateLimitConfig,
		Strategy:                 &DefaultStrategyConfig,
		Signature:                GetDefaultSignatureConfig(),
		MetaEvent:                &MetaEventConfiguration{IsEnabled: false},
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
				UID:       ulid.Make().String(),
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
	PausedEndpointStatus   EndpointStatus = "paused"
)

type (
	EndpointStatus string
	Secrets        []Secret
)

func (s *Secrets) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	var secrets []Secret
	err := json.Unmarshal(b, &secrets)
	if err != nil {
		return err
	}

	// Filter the deleted secrets out, start from the
	// last secret in the slice that hasn't been deleted
	ix := 0
	for i := range secrets {
		if secrets[i].DeletedAt.IsZero() {
			ix = i
			break
		}
	}

	*s = secrets[ix:]
	return nil
}

func (s Secrets) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}

	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type Endpoint struct {
	UID                string  `json:"uid" db:"id"`
	ProjectID          string  `json:"project_id" db:"project_id"`
	OwnerID            string  `json:"owner_id,omitempty" db:"owner_id"`
	TargetURL          string  `json:"target_url" db:"target_url"`
	Title              string  `json:"title" db:"title"`
	Secrets            Secrets `json:"secrets" db:"secrets"`
	AdvancedSignatures bool    `json:"advanced_signatures" db:"advanced_signatures"`
	Description        string  `json:"description" db:"description"`
	SlackWebhookURL    string  `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	SupportEmail       string  `json:"support_email,omitempty" db:"support_email"`
	AppID              string  `json:"-" db:"app_id"` // Deprecated but necessary for backward compatibility

	HttpTimeout string         `json:"http_timeout" db:"http_timeout"`
	RateLimit   int            `json:"rate_limit" db:"rate_limit"`
	Events      int64          `json:"events,omitempty" db:"event_count"`
	Status      EndpointStatus `json:"status" db:"status"`

	RateLimitDuration string                  `json:"rate_limit_duration" db:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication" db:"authentication"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func (e *Endpoint) FindSecret(secretID string) *Secret {
	for i := range e.Secrets {
		secret := &e.Secrets[i]
		if secret.UID == secretID {
			return secret
		}
	}
	return nil
}

type EndpointConfig struct {
	AdvancedSignatures bool                    `json:"advanced_signatures" db:"advanced_signatures"`
	Secrets            []Secret                `json:"secrets" db:"secrets"`
	RateLimit          *RateLimitConfiguration `json:"ratelimit" db:"ratelimit"`
	Authentication     *EndpointAuthentication `json:"authentication" db:"authentication"`
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
	Statistics      *ProjectStatistics `json:"statistics" db:"statistics"`

	RetainedEvents int `json:"retained_events" db:"retained_events"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type ProjectMetadata struct {
	RetainedEvents int `json:"retained_events" bson:"retained_events"`
}

type SignatureVersions []SignatureVersion

func (s *SignatureVersions) Scan(v interface{}) error {
	b, ok := v.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", v)
	}

	if string(b) == "null" {
		return nil
	}

	return json.Unmarshal(b, s)
}

func (s SignatureVersions) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type ProjectConfig struct {
	MaxIngestSize            uint64                        `json:"max_payload_read_size" db:"max_payload_read_size"`
	ReplayAttacks            bool                          `json:"replay_attacks_prevention_enabled" db:"replay_attacks_prevention_enabled"`
	IsRetentionPolicyEnabled bool                          `json:"retention_policy_enabled" db:"retention_policy_enabled"`
	DisableEndpoint          bool                          `json:"disable_endpoint" db:"disable_endpoint"`
	RetentionPolicy          *RetentionPolicyConfiguration `json:"retention_policy" db:"retention_policy"`
	RateLimit                *RateLimitConfiguration       `json:"ratelimit" db:"ratelimit"`
	Strategy                 *StrategyConfiguration        `json:"strategy" db:"strategy"`
	Signature                *SignatureConfiguration       `json:"signature" db:"signature"`
	MetaEvent                *MetaEventConfiguration       `json:"meta_event" db:"meta_event"`
}

func (p *ProjectConfig) GetRateLimitConfig() RateLimitConfiguration {
	if p.RateLimit != nil {
		return *p.RateLimit
	}
	return RateLimitConfiguration{}
}

func (p *ProjectConfig) GetStrategyConfig() StrategyConfiguration {
	if p.Strategy != nil {
		return *p.Strategy
	}
	return StrategyConfiguration{}
}

func (p *ProjectConfig) GetSignatureConfig() SignatureConfiguration {
	if p.Signature != nil {
		return *p.Signature
	}
	return SignatureConfiguration{}
}

func (p *ProjectConfig) GetRetentionPolicyConfig() RetentionPolicyConfiguration {
	if p.RetentionPolicy != nil {
		return *p.RetentionPolicy
	}
	return RetentionPolicyConfiguration{}
}

func (p *ProjectConfig) GetMetaEventConfig() MetaEventConfiguration {
	if p.MetaEvent != nil {
		return *p.MetaEvent
	}

	return MetaEventConfiguration{}
}

type RateLimitConfiguration struct {
	Count    int    `json:"count" db:"count"`
	Duration uint64 `json:"duration" db:"duration"`
}

type StrategyConfiguration struct {
	Type       StrategyProvider `json:"type" db:"type" valid:"optional~please provide a valid strategy type, in(linear|exponential)~unsupported strategy type"`
	Duration   uint64           `json:"duration" db:"duration" valid:"optional~please provide a valid duration in seconds,int"`
	RetryCount uint64           `json:"retry_count" db:"retry_count" valid:"optional~please provide a valid retry count,int"`
}

type SignatureConfiguration struct {
	Hash     string                         `json:"-" db:"hash"` // Deprecated
	Header   config.SignatureHeaderProvider `json:"header,omitempty" valid:"required~please provide a valid signature header"`
	Versions SignatureVersions              `json:"versions" db:"versions"`
}

type SignatureVersion struct {
	UID       string       `json:"uid" db:"id"`
	Hash      string       `json:"hash,omitempty" db:"hash" valid:"required~please provide a valid hash,supported_hash~unsupported hash type"`
	Encoding  EncodingType `json:"encoding" db:"encoding" valid:"required~please provide a valid signature header"`
	CreatedAt time.Time    `json:"created_at,omitempty" db:"created_at" swaggertype:"string"`
}

type MetaEventConfiguration struct {
	IsEnabled bool           `json:"is_enabled" db:"is_enabled"`
	Type      MetaEventType  `json:"type" db:"type" valid:"optional, in(http|pub_sub)~unsupported meta event type"`
	EventType pq.StringArray `json:"event_type" db:"event_type"`
	URL       string         `json:"url" db:"url"`
	Secret    string         `json:"secret" db:"secret"`
	PubSub    *PubSubConfig  `json:"pub_sub" db:"pub_sub"`
}

type RetentionPolicyConfiguration struct {
	Policy string `json:"policy" db:"policy" valid:"required~please provide a valid retention policy"`
}

type ProjectStatistics struct {
	MessagesSent       int64 `json:"messages_sent" db:"messages_sent"`
	TotalEndpoints     int64 `json:"total_endpoints" db:"total_endpoints"`
	TotalSubscriptions int64 `json:"total_subscriptions" db:"total_subscriptions"`
	TotalSources       int64 `json:"total_sources" db:"total_sources"`
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
	ErrSecretNotFound                = errors.New("secret not found")
	ErrMetaEventNotFound             = errors.New("meta event not found")
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

type EndpointMetadata []*Endpoint

func (s *EndpointMetadata) Scan(v interface{}) error {
	b, ok := v.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", v)
	}

	if string(b) == "null" {
		return nil
	}

	return json.Unmarshal(b, s)
}

// Event defines a payload to be sent to an application
type Event struct {
	UID              string    `json:"uid" db:"id"`
	EventType        EventType `json:"event_type" db:"event_type"`
	MatchedEndpoints int       `json:"matched_endpoints" db:"matched_enpoints"` // TODO(all) remove this field

	SourceID         string                `json:"source_id,omitempty" db:"source_id"`
	AppID            string                `json:"app_id,omitempty" db:"app_id"` // Deprecated
	ProjectID        string                `json:"project_id,omitempty" db:"project_id"`
	Endpoints        pq.StringArray        `json:"endpoints" db:"endpoints"`
	Headers          httpheader.HTTPHeader `json:"headers" db:"headers"`
	EndpointMetadata EndpointMetadata      `json:"endpoint_metadata,omitempty" db:"endpoint_metadata"`
	Source           *Source               `json:"source_metadata,omitempty" db:"source_metadata"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data,omitempty" db:"data"`
	Raw  string          `json:"raw,omitempty" db:"raw"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func (e *Event) GetRawHeaders() interface{} {
	h := map[string]interface{}{}
	for k, v := range e.Headers {
		h[k] = v[0]
	}
	return h
}

func (e *Event) GetRawHeadersJSON() ([]byte, error) {
	h := map[string]interface{}{}
	for k, v := range e.Headers {
		h[k] = v[0]
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

func (m *Metadata) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return nil
}

func (m *Metadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type EventIntervalData struct {
	Interval  int64  `json:"index" db:"index"`
	Time      string `json:"date" db:"total_time"`
	GroupStub string `json:"-" db:"group_only"` // ugnore
}

type EventInterval struct {
	Data  EventIntervalData `json:"data" db:"data"`
	Count uint64            `json:"count" db:"count"`
}

type DeliveryAttempt struct {
	UID        string `json:"uid" db:"id"`
	MsgID      string `json:"msg_id" db:"msg_id"`
	URL        string `json:"url" db:"url"`
	Method     string `json:"method" db:"method"`
	EndpointID string `json:"endpoint_id" db:"endpoint_id"`
	APIVersion string `json:"api_version" db:"api_version"`

	IPAddress        string     `json:"ip_address,omitempty" db:"ip_address"`
	RequestHeader    HttpHeader `json:"request_http_header,omitempty" db:"request_http_header"`
	ResponseHeader   HttpHeader `json:"response_http_header,omitempty" db:"response_http_header"`
	HttpResponseCode string     `json:"http_status,omitempty" db:"http_status"`
	ResponseData     string     `json:"response_data,omitempty" db:"response_data"`
	Error            string     `json:"error,omitempty" db:"error"`
	Status           bool       `json:"status,omitempty" db:"statu,"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type DeliveryAttempts []DeliveryAttempt

func (h *DeliveryAttempts) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &h); err != nil {
		return err
	}

	return nil
}

func (h DeliveryAttempts) Value() (driver.Value, error) {
	if h == nil {
		return nil, nil
	}

	b, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	return b, nil
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
	Source   *Source   `json:"source_metadata,omitempty" db:"source_metadata"`
	Device   *Device   `json:"device_metadata,omitempty" db:"device_metadata"`

	DeliveryAttempts DeliveryAttempts    `json:"-" db:"attempts"`
	Status           EventDeliveryStatus `json:"status" db:"status"`
	Metadata         *Metadata           `json:"metadata" db:"metadata"`
	CLIMetadata      *CLIMetadata        `json:"cli_metadata" db:"cli_metadata"`
	Description      string              `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time           `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt        time.Time           `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt        null.Time           `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type CLIMetadata struct {
	EventType string `json:"event_type" db:"event_type"`
	SourceID  string `json:"source_id" db:"source_id"`
}

func (m *CLIMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return nil
}

func (m *CLIMetadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type APIKey struct {
	UID       string    `json:"uid" db:"id"`
	MaskID    string    `json:"mask_id,omitempty" db:"mask_id"`
	Name      string    `json:"name" db:"name"`
	Role      auth.Role `json:"role" db:"role"`
	Hash      string    `json:"hash,omitempty" db:"hash"`
	Salt      string    `json:"salt,omitempty" db:"salt"`
	Type      KeyType   `json:"key_type" db:"key_type"`
	UserID    string    `json:"user_id" db:"user_id"`
	ExpiresAt null.Time `json:"expires_at,omitempty" db:"expires_at"`
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
	DeviceID   string           `json:"-" db:"device_id"`

	Source   *Source   `json:"source_metadata" db:"source_metadata"`
	Endpoint *Endpoint `json:"endpoint_metadata" db:"endpoint_metadata"`
	Device   *Device   `json:"device_metadata" db:"device_metadata"`

	// subscription config
	AlertConfig     *AlertConfiguration     `json:"alert_config,omitempty" db:"alert_config"`
	RetryConfig     *RetryConfiguration     `json:"retry_config,omitempty" db:"retry_config"`
	FilterConfig    *FilterConfiguration    `json:"filter_config,omitempty" db:"filter_config"`
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty" db:"rate_limit_config"`

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

	return FilterConfiguration{
		EventTypes: []string{},
		Filter: FilterSchema{
			Headers: M{},
			Body:    M{},
		},
	}
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
	URL            string          `json:"url" db:"-"`
	Type           SourceType      `json:"type" db:"type"`
	Provider       SourceProvider  `json:"provider" db:"provider"`
	IsDisabled     bool            `json:"is_disabled" db:"is_disabled"`
	VerifierID     string          `json:"-" db:"source_verifier_id"`
	Verifier       *VerifierConfig `json:"verifier" db:"verifier"`
	ProviderConfig *ProviderConfig `json:"provider_config" db:"provider_config"`
	ForwardHeaders pq.StringArray  `json:"forward_headers" db:"forward_headers"`
	PubSub         *PubSubConfig   `json:"pub_sub" db:"pub_sub"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type PubSubConfig struct {
	Type    PubSubType          `json:"type" db:"type"`
	Workers int                 `json:"workers" db:"workers"`
	Sqs     *SQSPubSubConfig    `json:"sqs" db:"sqs"`
	Google  *GooglePubSubConfig `json:"google" db:"google"`
}

func (p *PubSubConfig) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	var ps PubSubConfig
	err := json.Unmarshal(b, &ps)
	if err != nil {
		return err
	}

	*p = ps
	return nil
}

func (p PubSubConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type SQSPubSubConfig struct {
	AccessKeyID   string `json:"access_key_id" db:"access_key_id"`
	SecretKey     string `json:"secret_key" db:"secret_key"`
	DefaultRegion string `json:"default_region" db:"default_region"`
	QueueName     string `json:"queue_name" db:"queue_name"`
}

type GooglePubSubConfig struct {
	SubscriptionID string `json:"subscription_id" db:"subscription_id"`
	ServiceAccount []byte `json:"service_account" db:"service_account"`
	ProjectID      string `json:"project_id" db:"project_id"`
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
	Type       StrategyProvider `json:"type,omitempty" db:"type" valid:"supported_retry_strategy~please provide a valid retry strategy type"`
	Duration   uint64           `json:"duration,omitempty" db:"duration" valid:"duration~please provide a valid time duration"`
	RetryCount uint64           `json:"retry_count" db:"retry_count" valid:"int~please provide a valid retry count"`
}

type AlertConfiguration struct {
	Count     int    `json:"count" db:"count"`
	Threshold string `json:"threshold" db:"threshold" valid:"duration~please provide a valid time duration"`
}

type FilterConfiguration struct {
	EventTypes pq.StringArray `json:"event_types" db:"event_types"`
	Filter     FilterSchema   `json:"filter" db:"filter"`
}

type M map[string]interface{}

func (h M) Map() map[string]interface{} {
	m := map[string]interface{}{}
	x := map[string]interface{}(h)
	for k, v := range x {
		m[k] = v
	}
	return h
}

func (h *M) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	return json.Unmarshal(b, h)
}

func (h M) Value() (driver.Value, error) {
	if h == nil {
		return []byte("{}"), nil
	}

	b, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type FilterSchema struct {
	Headers M `json:"headers" db:"headers"`
	Body    M `json:"body" db:"body"`
}

type ProviderConfig struct {
	Twitter *TwitterProviderConfig `json:"twitter" db:"twitter"`
}

type TwitterProviderConfig struct {
	CrcVerifiedAt null.Time `json:"crc_verified_at" db:"crc_verified_at"`
}

type VerifierConfig struct {
	Type      VerifierType `json:"type,omitempty" db:"type" valid:"supported_verifier~please provide a valid verifier type"`
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
	UID            string      `json:"uid" db:"id"`
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
	UID              string       `json:"uid" db:"id"`
	OrganisationID   string       `json:"organisation_id" db:"organisation_id"`
	OrganisationName string       `json:"organisation_name,omitempty" db:"-"`
	InviteeEmail     string       `json:"invitee_email" db:"invitee_email"`
	Token            string       `json:"token" db:"token"`
	Role             auth.Role    `json:"role" db:"role"`
	Status           InviteStatus `json:"status" db:"status"`
	ExpiresAt        time.Time    `json:"-" db:"expires_at"`
	CreatedAt        time.Time    `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt        time.Time    `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt        null.Time    `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type PortalLink struct {
	UID               string           `json:"uid" db:"id"`
	Name              string           `json:"name" db:"name"`
	ProjectID         string           `json:"project_id" db:"project_id"`
	Token             string           `json:"-" db:"token"`
	Endpoints         pq.StringArray   `json:"endpoints" db:"endpoints"`
	EndpointsMetadata EndpointMetadata `json:"endpoints_metadata" db:"endpoints_metadata"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at,omitempty" swaggertype:"string"`
}

// Deprecated
type Application struct {
	UID             string `json:"uid" db:"id"`
	ProjectID       string `json:"project_id" db:"project_id"`
	Title           string `json:"name" db:"title"`
	SupportEmail    string `json:"support_email,omitempty" db:"support_email"`
	SlackWebhookURL string `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	IsDisabled      bool   `json:"is_disabled,omitempty" db:"is_disabled"`

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

type MetaEvent struct {
	UID       string              `json:"uid" db:"id"`
	ProjectID string              `json:"project_id" db:"project_id"`
	EventType string              `json:"event_type" db:"event_type"`
	Metadata  *Metadata           `json:"metadata" db:"metadata"`
	Attempt   *MetaEventAttempt   `json:"attempt" db:"attempt"`
	Status    EventDeliveryStatus `json:"status" db:"status"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

type MetaEventPayload struct {
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
}

type MetaEventAttempt struct {
	RequestHeader  HttpHeader `json:"request_http_header" db:"request_http_header"`
	ResponseHeader HttpHeader `json:"response_http_header" db:"response_http_header"`
	ResponseData   string     `json:"response_data,omitempty" db:"response_data"`
}

func (m *MetaEventAttempt) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return nil
}

func (m *MetaEventAttempt) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return b, nil
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
