package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/limiter"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/pkg/httpheader"

	"github.com/frain-dev/convoy/pkg/url"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
)

func ProcessEventDelivery(endpointRepo datastore.EndpointRepository, eventDeliveryRepo datastore.EventDeliveryRepository,
	projectRepo datastore.ProjectRepository, q queue.Queuer, rateLimiter limiter.RateLimiter, dispatch *net.Dispatcher, attemptsRepo datastore.DeliveryAttemptsRepository,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		var data EventDelivery
		defer func() {
			// retrieve the value of err
			if err == nil {
				return
			}

			// set the error to nil, so it's removed from the event queue
			err = nil

			job := &queue.Job{
				Payload: t.Payload(),
				Delay:   defaultEventDelay,
				ID:      data.EventDeliveryID,
			}

			// write it to the retry queue.
			deferErr := q.Write(convoy.RetryEventProcessor, convoy.RetryEventQueue, job)
			if deferErr != nil {
				log.FromContext(ctx).WithError(deferErr).Error("[asynq]: an error occurred sending event delivery to the retry queue")
			}
		}()

		err = msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			err = json.Unmarshal(t.Payload(), &data)
			if err != nil {
				return &DeliveryError{Err: err}
			}
		}

		cfg, err := config.Get()
		if err != nil {
			return &DeliveryError{Err: err}
		}

		eventDelivery, err := eventDeliveryRepo.FindEventDeliveryByIDSlim(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			return &DeliveryError{Err: err}
		}
		eventDelivery.Metadata.MaxRetrySeconds = cfg.MaxRetrySeconds

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		project, err := projectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			return &DeliveryError{Err: err}
		}

		endpoint, err := endpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				eventDelivery.Description = datastore.ErrEndpointNotFound.Error()
				err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
				if err != nil {
					log.WithError(err).Error("failed to update event delivery status to discarded")
				}

				return nil
			}

			return &DeliveryError{Err: err}
		}

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			return nil
		}

		err = rateLimiter.AllowWithDuration(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			log.FromContext(ctx).WithFields(map[string]interface{}{"event_delivery_id": data.EventDeliveryID}).
				WithError(err).
				Debugf("too many events to %s, limit of %v reqs/%v has been reached", endpoint.Url, endpoint.RateLimit, time.Duration(endpoint.RateLimitDuration)*time.Second)

			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			return &DeliveryError{Err: err}
		}

		done := true

		if eventDelivery.Status == datastore.SuccessEventStatus {
			log.Debugf("endpoint %s already merged with message %s\n", endpoint.Url, eventDelivery.UID)
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				return &DeliveryError{Err: err}
			}

			log.Debugf("endpoint %s is inactive, failing to send.", endpoint.Url)
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			return &DeliveryError{Err: err}
		}

		targetURL := endpoint.Url
		if !util.IsStringEmpty(eventDelivery.URLQueryParams) {
			targetURL, err = url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
			if err != nil {
				log.WithError(err).Error("failed to concat url query params")
				return &DeliveryError{Err: err}
			}
		}

		attemptStatus := false
		start := time.Now()

		if project.Config.AddEventIDTraceHeaders {
			if eventDelivery.Headers == nil {
				eventDelivery.Headers = httpheader.HTTPHeader{}
			}
			eventDelivery.Headers["X-Convoy-EventDelivery-ID"] = []string{eventDelivery.UID}
			eventDelivery.Headers["X-Convoy-Event-ID"] = []string{eventDelivery.EventID}
		}

		var httpDuration time.Duration
		if endpoint.HttpTimeout == 0 {
			httpDuration = convoy.HTTP_TIMEOUT_IN_DURATION
		} else {
			httpDuration = time.Duration(endpoint.HttpTimeout) * time.Second
		}
		resp, err := dispatch.SendRequest(ctx, targetURL, string(convoy.HttpPost), sig.Payload, project.Config.Signature.Header.String(), header, int64(cfg.MaxResponseSize), eventDelivery.Headers, eventDelivery.IdempotencyKey, httpDuration)

		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(start)
		// log request details
		requestLogger := log.FromContext(ctx).WithFields(log.Fields{
			"status":          status,
			"uri":             targetURL,
			"method":          convoy.HttpPost,
			"duration":        duration,
			"eventDeliveryID": eventDelivery.UID,
		})

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			requestLogger.Debugf("%s sent", eventDelivery.UID)
			attemptStatus = true

			eventDelivery.Status = datastore.SuccessEventStatus
			eventDelivery.Description = ""
			eventDelivery.LatencySeconds = time.Since(eventDelivery.GetLatencyStartTime()).Seconds()

			// register latency
			mm := metrics.GetDPInstance()
			mm.RecordLatency(eventDelivery)

		} else {
			requestLogger.Errorf("%s", eventDelivery.UID)
			done = false

			eventDelivery.Status = datastore.RetryEventStatus

			nextTime := time.Now().Add(delayDuration)
			eventDelivery.Metadata.NextSendTime = nextTime
			attempts := eventDelivery.Metadata.NumTrials + 1

			log.FromContext(ctx).Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", eventDelivery.UID,
				nextTime.Format(time.ANSIC), eventDelivery.Metadata.Strategy, eventDelivery.Metadata.IntervalSeconds, attempts, eventDelivery.Metadata.RetryLimit)
		}

		// Request failed but statusCode is 200 <= x <= 299
		if err != nil {
			log.Errorf("%s failed. Reason: %s", eventDelivery.UID, err)
		}

		if done && endpoint.Status == datastore.PendingEndpointStatus && project.Config.DisableEndpoint {
			endpointStatus := datastore.ActiveEndpointStatus
			err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}

			// send endpoint reactivation notification
			err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, q, false, resp.Error, string(resp.Body), resp.StatusCode)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error("failed to send notification")
			}
		}

		if !done && endpoint.Status == datastore.PendingEndpointStatus && project.Config.DisableEndpoint {
			endpointStatus := datastore.InactiveEndpointStatus
			err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
			if err != nil {
				log.FromContext(ctx).Errorf("Failed to reactivate endpoint after successful retry")
			}
		}

		attempt := parseAttemptFromResponse(eventDelivery, endpoint, resp, attemptStatus)

		eventDelivery.Metadata.NumTrials++

		if eventDelivery.Metadata.NumTrials >= eventDelivery.Metadata.RetryLimit {
			if done {
				if eventDelivery.Status != datastore.SuccessEventStatus {
					log.Errorln("an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					eventDelivery.Status = datastore.FailureEventStatus
				}
			} else {
				log.Errorf("%s retry limit exceeded ", eventDelivery.UID)
				eventDelivery.Description = "Retry limit exceeded"
				eventDelivery.Status = datastore.FailureEventStatus
			}

			if endpoint.Status != datastore.PendingEndpointStatus && project.Config.DisableEndpoint {
				endpointStatus := datastore.InactiveEndpointStatus

				err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					log.WithError(err).Error("failed to deactivate endpoint after failed retry")
				}

				// send endpoint deactivation notification
				err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, q, true, resp.Error, string(resp.Body), resp.StatusCode)
				if err != nil {
					log.WithError(err).Error("failed to send notification")
				}
			}
		}

		err = attemptsRepo.CreateDeliveryAttempt(ctx, &attempt)
		if err != nil {
			log.WithError(err).Error("failed to create delivery attempt", eventDelivery.UID)
			return &DeliveryError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error())}
		}

		err = eventDeliveryRepo.UpdateEventDeliveryMetadata(ctx, project.UID, *eventDelivery)
		if err != nil {
			log.WithError(err).Error("failed to update message ", eventDelivery.UID)
			return &DeliveryError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error())}
		}

		if !done && eventDelivery.Metadata.NumTrials < eventDelivery.Metadata.RetryLimit {
			errS := "nil"
			if err != nil {
				errS = err.Error()
			}
			return &DeliveryError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, errS)}
		}

		return nil
	}
}
