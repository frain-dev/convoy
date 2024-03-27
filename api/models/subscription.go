package models

import (
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/lib/pq"
	"net/http"
	"time"
)

type CreateSubscription struct {
	Name       string `json:"name" valid:"required~please provide a valid subscription name"`
	SourceID   string `json:"source_id"`
	AppID      string `json:"app_id"` // Deprecated but necessary for backward compatibility
	EndpointID string `json:"endpoint_id" valid:"required~please provide a valid endpoint id"`
	Function   string `json:"function"`

	AlertConfig     *AlertConfiguration     `json:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration     `json:"retry_config,omitempty"`
	FilterConfig    *FilterConfiguration    `json:"filter_config,omitempty"`
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty"`
}

func (cs *CreateSubscription) Validate() error {
	return util.Validate(cs)
}

type UpdateSubscription struct {
	Name       string `json:"name,omitempty"`
	AppID      string `json:"app_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	EndpointID string `json:"endpoint_id,omitempty"`
	Function   string `json:"function"`

	AlertConfig     *AlertConfiguration     `json:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration     `json:"retry_config,omitempty"`
	FilterConfig    *FilterConfiguration    `json:"filter_config,omitempty"`
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
	Request FilterSchema `json:"request"`
	Schema  FilterSchema `json:"schema"`
}

type AlertConfiguration struct {
	Count     int    `json:"count"`
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
	Type            datastore.StrategyProvider `json:"type,omitempty" valid:"supported_retry_strategy~please provide a valid retry strategy type"`
	Duration        string                     `json:"duration,omitempty" valid:"duration~please provide a valid time duration"`
	IntervalSeconds uint64                     `json:"interval_seconds" valid:"int~please provide a valid interval seconds"`
	RetryCount      uint64                     `json:"retry_count" valid:"int~please provide a valid retry count"`
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
	EventTypes pq.StringArray `json:"event_types"`
	Filter     FS             `json:"filter"`
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

type FunctionRequest struct {
	Payload  map[string]any `json:"payload"`
	Function string         `json:"function"`
	Type     string         `json:"type"`
}

type FunctionResponse struct {
	Payload any      `json:"payload"`
	Log     []string `json:"log"`
}

type DynamicSubscription struct {
	Name            string                  `json:"name"`
	AlertConfig     *AlertConfiguration     `json:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration     `json:"retry_config,omitempty"`
	FilterConfig    *FilterConfiguration    `json:"filter_config,omitempty"`
	RateLimitConfig *RateLimitConfiguration `json:"rate_limit_config,omitempty"`
}

type SubscriptionResponse struct {
	*datastore.Subscription
}
