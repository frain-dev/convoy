package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	tracer2 "github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/limiter"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/pkg/httpheader"

	"github.com/frain-dev/convoy/pkg/url"

	"github.com/frain-dev/convoy/pkg/signature"
	"github.com/oklog/ulid/v2"

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

var (
	ErrDeliveryAttemptFailed = errors.New("error sending event")
	ErrRateLimit             = errors.New("rate limit error")
	defaultDelay             = 10 * time.Second
	defaultEventDelay        = 120 * time.Second
)

func ProcessRetryEventDelivery(endpointRepo datastore.EndpointRepository, eventDeliveryRepo datastore.EventDeliveryRepository, licenser license.Licenser, projectRepo datastore.ProjectRepository, q queue.Queuer, rateLimiter limiter.RateLimiter, dispatch *net.Dispatcher, attemptsRepo datastore.DeliveryAttemptsRepository, circuitBreakerManager *circuit_breaker.CircuitBreakerManager, featureFlag *fflag.FFlag, tracerBackend tracer2.Backend) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data EventDelivery

		err := msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			innerErr := json.Unmarshal(t.Payload(), &data)
			if innerErr != nil {
				return &EndpointError{Err: innerErr, delay: defaultEventDelay}
			}
		}

		cfg, err := config.Get()
		if err != nil {
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		eventDelivery, err := eventDeliveryRepo.FindEventDeliveryByID(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		project, err := projectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		endpoint, err := endpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				eventDelivery.Description = datastore.ErrEndpointNotFound.Error()
				innerErr := eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
				if innerErr != nil {
					log.WithError(innerErr).Error("failed to update event delivery status to discarded")
				}

				return nil
			}

			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			return nil
		}

		err = rateLimiter.AllowWithDuration(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			log.FromContext(ctx).WithFields(map[string]interface{}{"event_delivery id": data.EventDeliveryID}).
				WithError(err).
				Debugf("too many events to %s, limit of %v reqs/%v has been reached", endpoint.Url, endpoint.RateLimit, time.Duration(endpoint.RateLimitDuration)*time.Second)

			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		if featureFlag.CanAccessFeature(fflag.CircuitBreaker) && licenser.CircuitBreaking() {
			breakerErr := circuitBreakerManager.CanExecute(ctx, endpoint.UID)
			if breakerErr != nil {
				return &CircuitBreakerError{Err: breakerErr}
			}

			// check the circuit breaker state so we can disable the endpoint
			cb, breakerErr := circuitBreakerManager.GetCircuitBreaker(ctx, endpoint.UID)
			if breakerErr != nil {
				return &CircuitBreakerError{Err: breakerErr}
			}

			if cb != nil {
				if cb.ConsecutiveFailures > circuitBreakerManager.GetConfig().ConsecutiveFailureThreshold {
					endpointStatus := datastore.InactiveEndpointStatus

					breakerErr = endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
					if breakerErr != nil {
						log.WithError(breakerErr).Error("failed to deactivate endpoint after failed retry")
					}
				}
			}
		}

		err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		var attempt datastore.DeliveryAttempt
		done := true

		if eventDelivery.Status == datastore.SuccessEventStatus {
			log.Debugf("endpoint %s already merged with message %s\n", endpoint.Url, eventDelivery.UID)
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultEventDelay}
			}

			log.Debugf("endpoint %s is inactive, failing to send.", endpoint.Url)
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		targetURL := endpoint.Url
		if !util.IsStringEmpty(eventDelivery.URLQueryParams) {
			targetURL, err = url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
			if err != nil {
				log.WithError(err).Error("failed to concat url query params")
				return &EndpointError{Err: err, delay: defaultEventDelay}
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
		if endpoint.HttpTimeout == 0 || !licenser.AdvancedEndpointMgmt() {
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
			eventDelivery.LatencySeconds = time.Since(eventDelivery.CreatedAt).Seconds()
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
		tracerBackend.Capture(project, targetURL, resp, duration)

		// Request failed but statusCode is 200 <= x <= 299
		if err != nil {
			log.Errorf("%s failed. Reason: %s", eventDelivery.UID, err)
		}

		if done && endpoint.Status == datastore.PendingEndpointStatus && project.Config.DisableEndpoint && !licenser.CircuitBreaking() {
			endpointStatus := datastore.ActiveEndpointStatus
			err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}

			if licenser.AdvancedEndpointMgmt() {
				// send endpoint reactivation notification
				err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, q, false, resp.Error, string(resp.Body), resp.StatusCode)
				if err != nil {
					log.FromContext(ctx).WithError(err).Error("failed to send notification")
				}
			}
		}

		if !done && endpoint.Status == datastore.PendingEndpointStatus && project.Config.DisableEndpoint && !licenser.CircuitBreaking() {
			endpointStatus := datastore.InactiveEndpointStatus
			err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
			if err != nil {
				log.FromContext(ctx).Errorf("Failed to reactivate endpoint after successful retry")
			}
		}

		attempt = parseAttemptFromResponse(eventDelivery, endpoint, resp, attemptStatus)

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

			if endpoint.Status != datastore.PendingEndpointStatus && project.Config.DisableEndpoint && !licenser.CircuitBreaking() {
				endpointStatus := datastore.InactiveEndpointStatus

				err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					log.WithError(err).Error("failed to deactivate endpoint after failed retry")
				}

				if licenser.AdvancedEndpointMgmt() {
					// send endpoint deactivation notification
					err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, q, true, resp.Error, string(resp.Body), resp.StatusCode)
					if err != nil {
						log.WithError(err).Error("failed to send notification")
					}
				}
			}
		}

		err = attemptsRepo.CreateDeliveryAttempt(ctx, &attempt)
		if err != nil {
			log.WithError(err).Errorf("failed to create delivery attempt for event delivery with id: %s", eventDelivery.UID)
			return &DeliveryError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error())}
		}

		err = eventDeliveryRepo.UpdateEventDeliveryMetadata(ctx, project.UID, eventDelivery)
		if err != nil {
			log.WithError(err).Error("failed to update message ", eventDelivery.UID)
			return &EndpointError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error()), delay: defaultEventDelay}
		}

		if !done && eventDelivery.Metadata.NumTrials < eventDelivery.Metadata.RetryLimit {
			errS := "nil"
			if err != nil {
				errS = err.Error()
			}
			return &EndpointError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, errS), delay: delayDuration}
		}

		return nil
	}
}

func newSignature(endpoint *datastore.Endpoint, g *datastore.Project, data json.RawMessage) *signature.Signature {
	s := &signature.Signature{Advanced: endpoint.AdvancedSignatures, Payload: data}

	for _, version := range g.Config.Signature.Versions {
		scheme := signature.Scheme{
			Hash:     version.Hash,
			Encoding: version.Encoding.String(),
		}

		for _, sc := range endpoint.Secrets {
			if sc.DeletedAt.IsZero() {
				// the secret has not been expired
				scheme.Secret = append(scheme.Secret, sc.Value)
			}
		}
		s.Schemes = append(s.Schemes, scheme)
	}

	return s
}

func parseAttemptFromResponse(m *datastore.EventDelivery, e *datastore.Endpoint, resp *net.Response, attemptStatus bool) datastore.DeliveryAttempt {
	responseHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.ResponseHeader)
	requestHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.RequestHeader)

	return datastore.DeliveryAttempt{
		UID:             ulid.Make().String(),
		URL:             resp.URL.String(),
		Method:          resp.Method,
		EventDeliveryId: m.UID,
		EndpointID:      e.UID,
		APIVersion:      convoy.GetVersion(),
		ProjectId:       m.ProjectID,

		IPAddress:        resp.IP,
		ResponseHeader:   *responseHeader,
		RequestHeader:    *requestHeader,
		HttpResponseCode: resp.Status,
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
