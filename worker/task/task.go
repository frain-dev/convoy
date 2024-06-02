package task

import (
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
