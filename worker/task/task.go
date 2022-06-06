package task

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/hibiken/asynq"
)

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
	return defaultDelay
}

func TestScheduleTask() func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		client := &http.Client{}
		id := uuid.NewString()
		b, err := json.Marshal(id)
		if err != nil {
			return err
		}
		req, _ := http.NewRequest("POST", "http://127.0.0.1:6000/", bytes.NewBuffer(b))
		req.Header.Add("Content-Type", "application/json")

		_, err = client.Do(req)
		if err != nil {
			log.WithError(err).Error("error sending request to API endpoint")
			return nil
		}
		return nil
	}
}
