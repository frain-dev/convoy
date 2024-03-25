package models

import (
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/lib/pq"
)

type CreateSubscription struct {
	// Subscription Nme
	Name string `json:"name" valid:"required~please provide a valid subscription name"`

	// Source Id
	SourceID string `json:"source_id"`

	AppID string `json:"app_id"` // Deprecated but necessary for backward compatibility

	// Destination endpoint ID
	EndpointID string `json:"endpoint_id" valid:"required~please provide a valid endpoint id"`

	// Convoy support mutating your request payload using a js function. Use this field
	// to specify a `transform` function for this purpose. See this[https://docs.getconvoy.io/product-manual/subscriptions#functions] for more
	Function string `json:"function"`

	// Alert configuration
	AlertConfig *AlertConfiguration `json:"alert_config,omitempty"`

	// Retry configuration
	RetryConfig *RetryConfiguration `json:"retry_config,omitempty"`

	// Filter configuration
	FilterConfig *FilterConfiguration `json:"filter_config,omitempty"`

	// Rate limit configuration
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty"`
}

func (cs *CreateSubscription) Validate() error {
	return util.Validate(cs)
}

type UpdateSubscription struct {
	// Subscription Nme
	Name string `json:"name,omitempty"`

	// Deprecated but necessary for backward compatibility
	AppID string `json:"app_id,omitempty"`

	// Source Id
	SourceID string `json:"source_id,omitempty"`

	// Destination endpoint ID
	EndpointID string `json:"endpoint_id,omitempty"`

	// Convoy support mutating your request payload using a js function. Use this field
	// to specify a `transform` function for this purpose. See this[https://docs.getconvoy.io/product-manual/subscriptions#functions] for more
	Function string `json:"function"`

	// Alert configuration
	AlertConfig *AlertConfiguration `json:"alert_config,omitempty"`

	// Retry configuration
	RetryConfig *RetryConfiguration `json:"retry_config,omitempty"`

	// Filter configuration
	FilterConfig *FilterConfiguration `json:"filter_config,omitempty"`

	// Rate limit configuration
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty"`
}

func (us *UpdateSubscription) Validate() error {
	return util.Validate(us)
}

type QueryListSubscription struct {
	// A list of endpointIDs to filter by
	EndpointIDs []string `json:"endpointId"`
	Pageable
}

type QueryListSubscriptionResponse struct {
	datastore.Pageable
	*datastore.FilterBy
}

func (qs *QueryListSubscription) Transform(r *http.Request) *QueryListSubscriptionResponse {
	if r == nil {
		return nil
	}

	return &QueryListSubscriptionResponse{
		Pageable: m.GetPageableFromContext(r.Context()),
		FilterBy: &datastore.FilterBy{
			EndpointIDs: getEndpointIDs(r),
		},
	}
}

type FilterSchema struct {
	Headers interface{} `json:"header"`
	Body    interface{} `json:"body"`
}

type TestFilter struct {
	// Same Request & Headers
	Request FilterSchema `json:"request"`

	// Sample test schema
	Schema FilterSchema `json:"schema"`
}

type TestWebhookFunction struct {
	Payload  map[string]interface{} `json:"payload"`
	Function string                 `json:"function"`
}

type AlertConfiguration struct {
	// Count
	Count int `json:"count"`

	// Threshold
	Threshold string `json:"threshold" valid:"duration~please provide a valid time duration"`
}

func (ac *AlertConfiguration) Transform() *datastore.AlertConfiguration {
	if ac == nil {
		return nil
	}

	return &datastore.AlertConfiguration{
		Count:     ac.Count,
		Threshold: ac.Threshold,
	}
}

type RetryConfiguration struct {
	// Retry Strategy type
	Type datastore.StrategyProvider `json:"type,omitempty" valid:"supported_retry_strategy~please provide a valid retry strategy type"`

	// TODO(all): remove IntervalSeconds & AlertConfig

	// Used to specify a valid Go time duration e.g 10s, 1h3m for how long to wait between event delivery retries
	Duration string `json:"duration,omitempty" valid:"duration~please provide a valid time duration"`

	// Used to specify a time in seconds for how long to wait between event delivery retries,
	IntervalSeconds uint64 `json:"interval_seconds" valid:"int~please provide a valid interval seconds"`

	// Used to specify the max number of retries
	RetryCount uint64 `json:"retry_count" valid:"int~please provide a valid retry count"`
}

func (rc *RetryConfiguration) Transform() (*datastore.RetryConfiguration, error) {
	if rc == nil {
		return nil, nil
	}

	strategyConfig := &datastore.RetryConfiguration{Type: rc.Type, RetryCount: rc.RetryCount}
	if !util.IsStringEmpty(rc.Duration) {
		interval, err := time.ParseDuration(rc.Duration)
		if err != nil {
			return nil, err
		}

		strategyConfig.Duration = uint64(interval.Seconds())
		return strategyConfig, nil
	}

	strategyConfig.Duration = rc.IntervalSeconds
	return strategyConfig, nil
}

type FilterConfiguration struct {
	// List of event types that the subscription should match
	EventTypes pq.StringArray `json:"event_types"`

	// Body & Header filters
	Filter FS `json:"filter"`
}

func (fc *FilterConfiguration) Transform() *datastore.FilterConfiguration {
	if fc == nil {
		return nil
	}

	return &datastore.FilterConfiguration{
		EventTypes: fc.EventTypes,
		Filter: datastore.FilterSchema{
			Headers: fc.Filter.Headers,
			Body:    fc.Filter.Body,
		},
	}
}

type FS struct {
	Headers datastore.M `json:"headers"`
	Body    datastore.M `json:"body"`
}

func (fs *FS) Transform() datastore.FilterSchema {
	return datastore.FilterSchema{
		Headers: fs.Headers,
		Body:    fs.Body,
	}
}

type SubscriptionFunctionResponse struct {
	Payload interface{} `json:"payload"`
	Log     []string    `json:"log"`
}

type SubscriptionResponse struct {
	*datastore.Subscription
}
