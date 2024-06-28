package task

import (
	"github.com/frain-dev/convoy/datastore"
	"time"

	"github.com/hibiken/asynq"
)

type DeliveryError struct {
	Err error
}

func (d *DeliveryError) Error() string {
	return d.Err.Error()
}

type EndpointError struct {
	delay time.Duration
	Err   error
}

func (e *EndpointError) Error() string {
	return e.Err.Error()
}

func (e *EndpointError) Delay() time.Duration {
	return e.delay
}

type RateLimitError struct {
	delay time.Duration
	Err   error
}

func (e *RateLimitError) Error() string {
	return e.Err.Error()
}

func (e *RateLimitError) Delay() time.Duration {
	return e.delay
}

func (e *RateLimitError) RateLimit() {
}

func GetRetryDelay(n int, err error, t *asynq.Task) time.Duration {
	if endpointError, ok := err.(*EndpointError); ok {
		return endpointError.Delay()
	}
	if rateLimitError, ok := err.(*RateLimitError); ok {
		return rateLimitError.Delay()
	}

	return asynq.DefaultRetryDelayFunc(n, err, t)
}

type SignatureValues struct {
	HMAC      string
	Timestamp string
}
type EventDelivery struct {
	EventDeliveryID string
	ProjectID       string
	AcknowledgedAt  time.Time
}

type EventDeliveryConfig struct {
	project      *datastore.Project
	subscription *datastore.Subscription
	endpoint     *datastore.Endpoint
}

type RetryConfig struct {
	Type       datastore.StrategyProvider
	Duration   uint64
	RetryCount uint64
}

type RateLimitConfig struct {
	Rate       int
	BucketSize int
}

func (ec *EventDeliveryConfig) RetryConfig() (*RetryConfig, error) {
	rc := &RetryConfig{}

	if ec.subscription.RetryConfig != nil {
		rc.Duration = ec.subscription.RetryConfig.Duration
		rc.RetryCount = ec.subscription.RetryConfig.RetryCount
		rc.Type = ec.subscription.RetryConfig.Type
	} else {
		rc.Duration = ec.project.Config.Strategy.Duration
		rc.RetryCount = ec.project.Config.Strategy.RetryCount
		rc.Type = ec.project.Config.Strategy.Type
	}

	return rc, nil
}

func (ec *EventDeliveryConfig) RateLimitConfig() *RateLimitConfig {
	rlc := &RateLimitConfig{}

	rlc.Rate = ec.endpoint.RateLimit
	rlc.BucketSize = int(ec.endpoint.RateLimitDuration)

	return rlc
}
