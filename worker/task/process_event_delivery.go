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
	"github.com/frain-dev/convoy/internal/pkg/cbenablement"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/httpheader"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/url"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
)

const (
	errMutualTLSFeatureUnavailable = "mutual TLS feature unavailable, please upgrade your license"
)

// defaultAsynqMaxRetries mirrors asynq's built-in default max retry count
// (asynq.DefaultMaxRetry, which the library does not export). The retry-queue
// task's asynq retry budget is never set below this value.
const defaultAsynqMaxRetries = 25

var errEndpointURLTemplateTargetMissing = errors.New("endpoint URL template requires a concrete target URL")

func resolveEventDeliveryTargetURL(endpoint *datastore.Endpoint, eventDelivery *datastore.EventDelivery) (string, error) {
	targetURL := eventDelivery.TargetURL
	if util.IsStringEmpty(targetURL) && url.ContainsTemplate(endpoint.Url) {
		return "", errEndpointURLTemplateTargetMissing
	}

	if util.IsStringEmpty(eventDelivery.URLQueryParams) {
		if !util.IsStringEmpty(targetURL) {
			return targetURL, nil
		}
		return endpoint.Url, nil
	}

	if !util.IsStringEmpty(targetURL) {
		return url.ConcatQueryParams(targetURL, eventDelivery.URLQueryParams)
	}

	return url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
}

//nolint:cyclop // Large function handling complex event delivery logic with many conditional branches
type EventDeliveryProcessorDeps struct {
	EndpointRepo               datastore.EndpointRepository
	EventDeliveryRepo          datastore.EventDeliveryRepository
	Licenser                   license.Licenser
	ProjectRepo                datastore.ProjectRepository
	Queue                      queue.Queuer
	RateLimiter                limiter.RateLimiter
	Dispatcher                 *net.Dispatcher
	AttemptsRepo               datastore.DeliveryAttemptsRepository
	CircuitBreakerManager      *circuit_breaker.CircuitBreakerManager
	CBEnablement               *cbenablement.Resolver
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	OAuth2TokenService         OAuth2TokenService
	Logger                     log.Logger
}

func ProcessEventDelivery(deps EventDeliveryProcessorDeps) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		// Start a new trace span for event delivery
		attributes := map[string]interface{}{
			"event.type": "event.delivery",
		}

		var data EventDelivery
		var delayDuration time.Duration
		// retryLimit caps how many times asynq retries the retry-queue task. It
		// stays nil until the event delivery is loaded; early failures fall back
		// to asynq's default budget.
		var retryLimit *int

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
				Payload:  t.Payload(),
				Delay:    delayDuration,
				ID:       data.EventDeliveryID,
				MaxRetry: retryLimit,
			}

			// write it to the retry queue.
			deferErr := deps.Queue.Write(ctx, convoy.RetryEventProcessor, convoy.RetryEventQueue, job)
			if deferErr != nil {
				deps.Logger.ErrorContext(ctx, "[asynq]: an error occurred sending event delivery to the retry queue", "error", deferErr)
			}
		}()

		err = msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			err = json.Unmarshal(t.Payload(), &data)
			if err != nil {
				tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
				return &DeliveryError{Err: err}
			}
		}

		attributes["event_delivery.id"] = data.EventDeliveryID
		attributes["project.id"] = data.ProjectID

		cfg, err := config.Get()
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}

		eventDelivery, err := deps.EventDeliveryRepo.FindEventDeliveryByIDSlim(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}
		eventDelivery.Metadata.MaxRetrySeconds = cfg.MaxRetrySeconds

		// Sync the retry-queue task's asynq retry budget with the configured retry
		// limit so large limits are not silently capped at asynq's default of 25.
		// We only ever raise the budget, never lower it: limits at or below the
		// default are already enforced by the NumTrials check below, and keeping
		// the default budget preserves headroom for transient pre-dispatch errors.
		// Those return an EndpointError that consumes an asynq retry without
		// advancing NumTrials, so a tighter budget could archive the task before
		// the configured attempts run. Rate-limit and circuit-breaker errors are
		// excluded from this budget by the consumer's IsFailure policy.
		rl := int(eventDelivery.Metadata.RetryLimit)
		if rl < defaultAsynqMaxRetries {
			rl = defaultAsynqMaxRetries
		}
		retryLimit = &rl

		delayDuration = retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		project, err := deps.ProjectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}
		if err = license.EnsureProjectEnabled(deps.Licenser, project.UID); err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}

		endpoint, err := deps.EndpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				eventDelivery.Description = datastore.ErrEndpointNotFound.Error()
				err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
				if err != nil {
					deps.Logger.ErrorContext(ctx, "failed to update event delivery status to discarded", "error", err)
				}

				tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
				return nil
			}

			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}

		attributes["endpoint.url"] = endpoint.Url
		attributes["endpoint.id"] = endpoint.UID
		attributes["event.id"] = eventDelivery.EventID

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			tracer.AddEvent(ctx, tracer.EventEventDeliverySuccess, attributes)
			return nil
		}

		err = deps.RateLimiter.AllowWithDuration(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			deps.Logger.DebugContext(ctx, "too many events, rate limit reached", "endpoint_url", endpoint.Url, "rate_limit", endpoint.RateLimit, "rate_limit_duration", time.Duration(endpoint.RateLimitDuration)*time.Second, "event_delivery_id", data.EventDeliveryID, "error", err)

			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		// Enforcement is gated by the same resolver as the sampler and display:
		// license + live enablement for this org (env folded into the instance base,
		// per-org override wins). The manager is always constructed now, so the
		// nil-guards are defensive (e.g. tests that omit them). They run first so the
		// licenser and cached resolver lookups only happen when CB wiring is present.
		if deps.CircuitBreakerManager != nil && deps.CBEnablement != nil &&
			deps.Licenser.CircuitBreaking() && deps.CBEnablement.EnabledForOrg(ctx, project.OrganisationID) {
			breakerErr := deps.CircuitBreakerManager.CanExecute(ctx, endpoint.UID)
			if breakerErr != nil {
				tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
				return &CircuitBreakerError{Err: breakerErr}
			}
		}

		err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}

		done := true

		if eventDelivery.Status == datastore.SuccessEventStatus {
			deps.Logger.DebugContext(ctx, "endpoint already merged with message", "endpoint_url", endpoint.Url, "event_delivery_uid", eventDelivery.UID)
			tracer.AddEvent(ctx, tracer.EventEventDeliverySuccess, attributes)
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
				return &DeliveryError{Err: err}
			}

			deps.Logger.DebugContext(ctx, "endpoint is inactive, failing to send", "endpoint_url", endpoint.Url)
			tracer.AddEvent(ctx, tracer.EventEventDeliveryDiscarded, attributes)
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
		}

		targetURL, err := resolveEventDeliveryTargetURL(endpoint, eventDelivery)
		if err != nil {
			if errors.Is(err, errEndpointURLTemplateTargetMissing) {
				eventDelivery.Description = err.Error()
				if updateErr := deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus); updateErr != nil {
					deps.Logger.ErrorContext(ctx, "failed to discard event delivery with unresolved endpoint URL template", "error", updateErr)
					tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
					return &DeliveryError{Err: updateErr}
				}
				tracer.AddEvent(ctx, tracer.EventEventDeliveryDiscarded, attributes)
				return nil
			}

			deps.Logger.ErrorContext(ctx, "failed to resolve event delivery target url", "error", err)
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return &DeliveryError{Err: err}
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
			oauth2Enabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, project.OrganisationID)
			if !oauth2Enabled {
				deps.Logger.WarnContext(ctx, "Endpoint has OAuth2 configured but feature flag is disabled, removing OAuth2 authorization header")
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
				deps.Logger.ErrorContext(ctx, errMutualTLSFeatureUnavailable)
				eventDelivery.Status = datastore.FailureEventStatus
				eventDelivery.Description = errMutualTLSFeatureUnavailable
				err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
				if err != nil {
					deps.Logger.ErrorContext(ctx, "failed to update event delivery status to failed", "error", err)
				}
				tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
				return nil // Return nil to avoid retrying
			}

			// Check feature flag for mTLS using project's organisation ID
			mtlsEnabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.MTLS, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, project.OrganisationID)
			if !mtlsEnabled {
				deps.Logger.WarnContext(ctx, "Endpoint has mTLS configured but feature flag is disabled, continuing without mTLS")
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
					deps.Logger.ErrorContext(ctx, "failed to load mTLS client certificate", "error", certErr)
					eventDelivery.Status = datastore.FailureEventStatus
					eventDelivery.Description = fmt.Sprintf("Invalid mTLS certificate: %v", certErr)
					err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
					if err != nil {
						deps.Logger.ErrorContext(ctx, "failed to update event delivery status to failed", "error", err)
					}
					tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
					return nil // Return nil to avoid retrying
				}
				mtlsCert = cert
			}
		}

		// When a project configures a custom request ID header, producers must supply
		// idempotency_key at publish time (any stable value); that value is sent on the
		// outbound header below.
		requestSentAt := time.Now()
		resp, err := deps.Dispatcher.SendWebhookWithMTLS(
			ctx,
			targetURL,
			sig.Payload,
			project.Config.Signature.Header.String(),
			header,
			int64(cfg.MaxResponseSize),
			eventDelivery.Headers,
			project.Config.GetRequestIDHeader().String(),
			eventDelivery.IdempotencyKey,
			httpDuration,
			contentType,
			mtlsCert,
		)
		responseReceivedAt := time.Now()

		// Missing idempotency key for a custom request ID header is deterministic; fail
		// closed and do not schedule retries.
		if errors.Is(err, datastore.ErrMissingIdempotencyKeyForCustomRequestIDHeader) {
			deps.Logger.ErrorContext(ctx, "event delivery missing idempotency key for custom request id header", "error", err, "event_delivery_uid", eventDelivery.UID)
			eventDelivery.Status = datastore.FailureEventStatus
			eventDelivery.Description = err.Error()
			if updateErr := deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus); updateErr != nil {
				deps.Logger.ErrorContext(ctx, "failed to update event delivery status to failed", "error", updateErr)
			}
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
			return nil
		}

		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(httpDispatchStart)
		// log request details
		logAttrs := []any{"status", status, "uri", targetURL, "method", convoy.HttpPost, "duration", duration, "eventDeliveryID", eventDelivery.UID}

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			deps.Logger.DebugContext(ctx, "event delivery sent", append(logAttrs, "event_delivery_uid", eventDelivery.UID)...)
			attemptStatus = true

			eventDelivery.Status = datastore.SuccessEventStatus
			eventDelivery.Description = ""
			eventDelivery.LatencySeconds = time.Since(eventDelivery.GetLatencyStartTime()).Seconds()

			// register latency
			mm := metrics.GetDPInstance(deps.Licenser)
			mm.RecordEndToEndLatency(eventDelivery)
		} else {
			deps.Logger.ErrorContext(ctx, "event delivery http error", append(logAttrs, "event_delivery_uid", eventDelivery.UID)...)
			done = false

			// For at-most-once delivery, only retry on network failures
			if eventDelivery.DeliveryMode == datastore.AtMostOnceDeliveryMode {
				if retryableForAtMostOnceDeliveryMode(resp.StatusCode) {
					// Network error - retry
					eventDelivery.Status = datastore.RetryEventStatus
					nextTime := time.Now().Add(delayDuration)
					eventDelivery.Metadata.NextSendTime = nextTime
					attempts := eventDelivery.Metadata.NumTrials + 1

					deps.Logger.ErrorContext(ctx, "event delivery retry scheduled", "event_delivery_uid", eventDelivery.UID, "next_send_time", nextTime.Format(time.ANSIC), "strategy", eventDelivery.Metadata.Strategy, "interval_seconds", eventDelivery.Metadata.IntervalSeconds, "attempt", attempts, "retry_limit", eventDelivery.Metadata.RetryLimit) //nolint:lll
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

				deps.Logger.ErrorContext(ctx, "event delivery retry scheduled", "event_delivery_uid", eventDelivery.UID, "next_send_time", nextTime.Format(time.ANSIC), "strategy", eventDelivery.Metadata.Strategy, "interval_seconds", eventDelivery.Metadata.IntervalSeconds, "attempt", attempts, "retry_limit", eventDelivery.Metadata.RetryLimit) //nolint:lll
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
			deps.Logger.ErrorContext(ctx, "event delivery failed", "event_delivery_uid", eventDelivery.UID, "error", err)
			tracer.AddEvent(ctx, tracer.EventEventDeliveryError, attributes)
		} else {
			tracer.AddEvent(ctx, tracer.EventEventDeliverySuccess, attributes)
		}

		attributes["project.id"] = project.UID
		attributes["endpoint.url"] = endpoint.Url
		attributes["endpoint.id"] = endpoint.UID
		attributes["event_delivery.id"] = eventDelivery.UID
		attributes["event.id"] = eventDelivery.EventID

		tracer.AddEvent(ctx, tracer.EventEventDeliveryInfo, attributes)

		respondedAt := time.Time{}
		if resp != nil && resp.StatusCode >= 100 {
			respondedAt = responseReceivedAt
		}
		attempt := parseAttemptFromResponse(eventDelivery, endpoint, resp, attemptStatus, requestSentAt, respondedAt)
		eventDelivery.Metadata.NumTrials++

		if eventDelivery.Metadata.NumTrials >= eventDelivery.Metadata.RetryLimit {
			if done {
				if eventDelivery.Status != datastore.SuccessEventStatus {
					deps.Logger.ErrorContext(ctx, "an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					eventDelivery.Status = datastore.FailureEventStatus
				}
			} else {
				deps.Logger.ErrorContext(ctx, "retry limit exceeded", "event_delivery_uid", eventDelivery.UID)
				eventDelivery.Description = "Retry limit exceeded"
				eventDelivery.Status = datastore.FailureEventStatus
			}

			if project.Config.DisableEndpoint && !deps.Licenser.CircuitBreaking() {
				endpointStatus := datastore.InactiveEndpointStatus

				err = deps.EndpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					deps.Logger.ErrorContext(ctx, "failed to deactivate endpoint after failed retry", "error", err)
				}

				if deps.Licenser.AdvancedEndpointMgmt() {
					failureMsg := ""
					responseBody := ""
					statusCode := 0
					if resp != nil {
						failureMsg = resp.Error
						responseBody = string(resp.Body)
						statusCode = resp.StatusCode
					}

					// send endpoint deactivation notification
					err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, deps.Queue, true, failureMsg, responseBody, statusCode, deps.Logger)
					if err != nil {
						deps.Logger.ErrorContext(ctx, "failed to send notification", "error", err)
					}
				}
			}
		}

		err = deps.AttemptsRepo.CreateDeliveryAttempt(ctx, &attempt)
		if err != nil {
			deps.Logger.ErrorContext(ctx, "failed to create delivery attempt", "event_delivery_uid", eventDelivery.UID, "response_data", attempt.ResponseData, "error", err)
			return &DeliveryError{Err: fmt.Errorf("%w: %w", ErrDeliveryAttemptFailed, err)}
		}

		err = deps.EventDeliveryRepo.UpdateEventDeliveryMetadata(ctx, project.UID, eventDelivery)
		if err != nil {
			deps.Logger.ErrorContext(ctx, "failed to update message", "error", err, "event_delivery_uid", eventDelivery.UID)
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
