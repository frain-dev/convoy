package task

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/url"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
)

const (
	errMutualTLSFeatureUnavailable = "mutual TLS feature unavailable, please upgrade your license"
)

//nolint:cyclop // Large function handling complex event delivery logic with many conditional branches
type EventDeliveryProcessorDeps struct {
	EndpointRepo          datastore.EndpointRepository
	EventDeliveryRepo     datastore.EventDeliveryRepository
	Licenser              license.Licenser
	ProjectRepo           datastore.ProjectRepository
	Queue                 queue.Queuer
	RateLimiter           limiter.RateLimiter
	Dispatcher            *net.Dispatcher
	AttemptsRepo          datastore.DeliveryAttemptsRepository
	CircuitBreakerManager *circuit_breaker.CircuitBreakerManager
	FeatureFlag           *fflag.FFlag
	FeatureFlagFetcher    fflag.FeatureFlagFetcher
	TracerBackend         tracer.Backend
	OAuth2TokenService    OAuth2TokenService
}

func ProcessEventDelivery(deps EventDeliveryProcessorDeps) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		// Start a new trace span for event delivery
		traceStartTime := time.Now()
		attributes := map[string]interface{}{
			"event.type": "event.delivery",
		}

		var data EventDelivery
		var delayDuration time.Duration

		defer func() {
			// retrieve the value of err
			if err == nil {
				return
			}

			// set the error to nil, so it's removed from the event queue
			err = nil

			if delayDuration == 0 {
				delayDuration = defaultEventDelay
			}

			job := &queue.Job{
				Payload: t.Payload(),
				Delay:   delayDuration,
				ID:      data.EventDeliveryID,
			}

			// write it to the retry queue.
			deferErr := deps.Queue.Write(convoy.RetryEventProcessor, convoy.RetryEventQueue, job)
			if deferErr != nil {
				log.FromContext(ctx).WithError(deferErr).Error("[asynq]: an error occurred sending event delivery to the retry queue")
			}
		}()

		err = msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			err = json.Unmarshal(t.Payload(), &data)
			if err != nil {
				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return &DeliveryError{Err: err}
			}
		}

		attributes["event_delivery.id"] = data.EventDeliveryID
		attributes["project.id"] = data.ProjectID

		cfg, err := config.Get()
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}

		eventDelivery, err := deps.EventDeliveryRepo.FindEventDeliveryByIDSlim(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}
		eventDelivery.Metadata.MaxRetrySeconds = cfg.MaxRetrySeconds

		delayDuration = retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		project, err := deps.ProjectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}

		endpoint, err := deps.EndpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				eventDelivery.Description = datastore.ErrEndpointNotFound.Error()
				err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
				if err != nil {
					log.FromContext(ctx).WithError(err).Error("failed to update event delivery status to discarded")
				}

				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return nil
			}

			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}

		attributes["endpoint.url"] = endpoint.Url
		attributes["endpoint.id"] = endpoint.UID
		attributes["event.id"] = eventDelivery.EventID

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			deps.TracerBackend.Capture(ctx, "event.delivery.success", attributes, traceStartTime, time.Now())
			return nil
		}

		err = deps.RateLimiter.AllowWithDuration(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			log.FromContext(ctx).WithFields(map[string]interface{}{"event_delivery_id": data.EventDeliveryID}).
				WithError(err).
				Debugf("too many events to %s, limit of %v reqs/%v has been reached", endpoint.Url, endpoint.RateLimit, time.Duration(endpoint.RateLimitDuration)*time.Second)

			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		if deps.FeatureFlag.CanAccessFeature(fflag.CircuitBreaker) && deps.Licenser.CircuitBreaking() {
			breakerErr := deps.CircuitBreakerManager.CanExecute(ctx, endpoint.UID)
			if breakerErr != nil {
				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return &CircuitBreakerError{Err: breakerErr}
			}
		}

		err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}

		done := true

		if eventDelivery.Status == datastore.SuccessEventStatus {
			log.FromContext(ctx).Debugf("endpoint %s already merged with message %s\n", endpoint.Url, eventDelivery.UID)
			deps.TracerBackend.Capture(ctx, "event.delivery.success", attributes, traceStartTime, time.Now())
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return &DeliveryError{Err: err}
			}

			log.FromContext(ctx).Debugf("endpoint %s is inactive, failing to send.", endpoint.Url)
			deps.TracerBackend.Capture(ctx, "event.delivery.discarded", attributes, traceStartTime, time.Now())
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
			return &DeliveryError{Err: err}
		}

		targetURL := endpoint.Url
		if !util.IsStringEmpty(eventDelivery.URLQueryParams) {
			targetURL, err = url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error("failed to concat url query params")
				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return &DeliveryError{Err: err}
			}
		}

		attemptStatus := false
		httpDispatchStart := time.Now()

		if project.Config.AddEventIDTraceHeaders {
			if eventDelivery.Headers == nil {
				eventDelivery.Headers = httpheader.HTTPHeader{}
			}
			eventDelivery.Headers["X-Convoy-EventDelivery-ID"] = []string{eventDelivery.UID}
			eventDelivery.Headers["X-Convoy-Event-ID"] = []string{eventDelivery.EventID}
		}

		// Check feature flag for OAuth2 if endpoint uses OAuth2 authentication
		if endpoint.Authentication != nil && endpoint.Authentication.Type == datastore.OAuth2Authentication {
			oauth2Enabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, deps.FeatureFlagFetcher, project.OrganisationID)
			if !oauth2Enabled {
				log.FromContext(ctx).Warn("Endpoint has OAuth2 configured but feature flag is disabled, removing OAuth2 authorization header")
				// Remove OAuth2 authorization header if feature flag is disabled
				if eventDelivery.Headers != nil {
					delete(eventDelivery.Headers, "Authorization")
				}
			}
		}

		var httpDuration time.Duration
		if endpoint.HttpTimeout == 0 || !deps.Licenser.AdvancedEndpointMgmt() {
			httpDuration = convoy.HTTP_TIMEOUT_IN_DURATION
		} else {
			httpDuration = time.Duration(endpoint.HttpTimeout) * time.Second
		}

		contentType := endpoint.ContentType
		if contentType == "" {
			contentType = "application/json"
		}

		// Load mTLS client certificate if configured
		var mtlsCert *tls.Certificate
		if endpoint.MtlsClientCert != nil {
			// Check license before using mTLS during delivery
			if !deps.Licenser.MutualTLS() {
				log.FromContext(ctx).Error(errMutualTLSFeatureUnavailable)
				eventDelivery.Status = datastore.FailureEventStatus
				eventDelivery.Description = errMutualTLSFeatureUnavailable
				err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
				if err != nil {
					log.FromContext(ctx).WithError(err).Error("failed to update event delivery status to failed")
				}
				deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
				return nil // Return nil to avoid retrying
			}

			// Check feature flag for mTLS using project's organisation ID
			mtlsEnabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.MTLS, deps.FeatureFlagFetcher, project.OrganisationID)
			if !mtlsEnabled {
				log.FromContext(ctx).Warn("Endpoint has mTLS configured but feature flag is disabled, continuing without mTLS")
				// Continue without mTLS if feature flag is disabled
				mtlsCert = nil
			} else {
				// Use cached certificate loading to avoid parsing on every request
				cert, certErr := config.LoadClientCertificateWithCache(
					endpoint.UID, // Use endpoint ID as cache key
					endpoint.MtlsClientCert.ClientCert,
					endpoint.MtlsClientCert.ClientKey,
				)
				if certErr != nil {
					// Fail fast on certificate errors (invalid or expired cert) to avoid needless retries
					log.FromContext(ctx).WithError(certErr).Error("failed to load mTLS client certificate")
					eventDelivery.Status = datastore.FailureEventStatus
					eventDelivery.Description = fmt.Sprintf("Invalid mTLS certificate: %v", certErr)
					err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
					if err != nil {
						log.FromContext(ctx).WithError(err).Error("failed to update event delivery status to failed")
					}
					deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
					return nil // Return nil to avoid retrying
				}
				mtlsCert = cert
			}
		}

		resp, err := deps.Dispatcher.SendWebhookWithMTLS(ctx, targetURL, sig.Payload, project.Config.Signature.Header.String(), header, int64(cfg.MaxResponseSize), eventDelivery.Headers, eventDelivery.IdempotencyKey, httpDuration, contentType, mtlsCert)

		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(httpDispatchStart)
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
			mm := metrics.GetDPInstance(deps.Licenser)
			mm.RecordEndToEndLatency(eventDelivery)
		} else {
			requestLogger.Errorf("%s", eventDelivery.UID)
			done = false

			// For at-most-once delivery, only retry on network failures
			if eventDelivery.DeliveryMode == datastore.AtMostOnceDeliveryMode {
				if retryableForAtMostOnceDeliveryMode(resp.StatusCode) {
					// Network error - retry
					eventDelivery.Status = datastore.RetryEventStatus
					nextTime := time.Now().Add(delayDuration)
					eventDelivery.Metadata.NextSendTime = nextTime
					attempts := eventDelivery.Metadata.NumTrials + 1

					log.FromContext(ctx).Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", eventDelivery.UID,
						nextTime.Format(time.ANSIC), eventDelivery.Metadata.Strategy, eventDelivery.Metadata.IntervalSeconds, attempts, eventDelivery.Metadata.RetryLimit)
				} else {
					// Got a response (even if it's an error status code) - mark as failed
					eventDelivery.Status = datastore.FailureEventStatus
					eventDelivery.Description = fmt.Sprintf("Endpoint returned status code %d", statusCode)
					done = true
				}
			} else {
				// At-least-once delivery - retry on any failure
				eventDelivery.Status = datastore.RetryEventStatus
				nextTime := time.Now().Add(delayDuration)
				eventDelivery.Metadata.NextSendTime = nextTime
				attempts := eventDelivery.Metadata.NumTrials + 1

				log.FromContext(ctx).Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", eventDelivery.UID,
					nextTime.Format(time.ANSIC), eventDelivery.Metadata.Strategy, eventDelivery.Metadata.IntervalSeconds, attempts, eventDelivery.Metadata.RetryLimit)
			}
		}

		// Update attributes with response info
		if resp != nil {
			attributes["response.status"] = resp.Status
			attributes["response.ip"] = resp.IP
			attributes["response.status_code"] = resp.StatusCode
			attributes["response.size_bytes"] = len(resp.Body)
		}

		// The request failed, but the statusCode is 200 <= x <= 299
		if err != nil {
			log.FromContext(ctx).Errorf("%s failed. Reason: %s", eventDelivery.UID, err)
			deps.TracerBackend.Capture(ctx, "event.delivery.error", attributes, traceStartTime, time.Now())
		} else {
			deps.TracerBackend.Capture(ctx, "event.delivery.success", attributes, traceStartTime, time.Now())
		}

		attributes["project.id"] = project.UID
		attributes["endpoint.url"] = endpoint.Url
		attributes["endpoint.id"] = endpoint.UID
		attributes["event_delivery.id"] = eventDelivery.UID
		attributes["event.id"] = eventDelivery.EventID

		deps.TracerBackend.Capture(ctx, "event.delivery.info", attributes, time.Now(), time.Now())

		attempt := parseAttemptFromResponse(eventDelivery, endpoint, resp, attemptStatus)
		eventDelivery.Metadata.NumTrials++

		if eventDelivery.Metadata.NumTrials >= eventDelivery.Metadata.RetryLimit {
			if done {
				if eventDelivery.Status != datastore.SuccessEventStatus {
					log.FromContext(ctx).Error("an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					eventDelivery.Status = datastore.FailureEventStatus
				}
			} else {
				log.FromContext(ctx).Errorf("%s retry limit exceeded ", eventDelivery.UID)
				eventDelivery.Description = "Retry limit exceeded"
				eventDelivery.Status = datastore.FailureEventStatus
			}

			if project.Config.DisableEndpoint && !deps.Licenser.CircuitBreaking() {
				endpointStatus := datastore.InactiveEndpointStatus

				err = deps.EndpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					log.FromContext(ctx).WithError(err).Error("failed to deactivate endpoint after failed retry")
				}

				if deps.Licenser.AdvancedEndpointMgmt() {
					// send endpoint deactivation notification
					err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, deps.Queue, true, resp.Error, string(resp.Body), resp.StatusCode)
					if err != nil {
						log.FromContext(ctx).WithError(err).Error("failed to send notification")
					}
				}
			}
		}

		err = deps.AttemptsRepo.CreateDeliveryAttempt(ctx, &attempt)
		if err != nil {
			log.FromContext(ctx).
				WithError(err).
				Errorf("failed to create delivery attempt for event delivery with id: %s and delivery attempt: %s", eventDelivery.UID, attempt.ResponseData)
			return &DeliveryError{Err: fmt.Errorf("%w: %w", ErrDeliveryAttemptFailed, err)}
		}

		err = deps.EventDeliveryRepo.UpdateEventDeliveryMetadata(ctx, project.UID, eventDelivery)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to update message ", eventDelivery.UID)
			return &DeliveryError{Err: fmt.Errorf("%w: %w", ErrDeliveryAttemptFailed, err)}
		}

		if !done && eventDelivery.Metadata.NumTrials < eventDelivery.Metadata.RetryLimit {
			errS := "nil"
			if err != nil {
				errS = err.Error()
			}
			return &DeliveryError{Err: fmt.Errorf("%w: %s", ErrDeliveryAttemptFailed, errS)}
		}

		return nil
	}
}

func retryableForAtMostOnceDeliveryMode(statusCode int) bool {
	return statusCode < 100
}
