package task

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/signature"
	"github.com/frain-dev/convoy/pkg/url"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrDeliveryAttemptFailed = errors.New("error sending event")
	ErrRateLimit             = errors.New("rate limit error")
	defaultDelay             = 10 * time.Second
	defaultEventDelay        = 120 * time.Second
)

//nolint:cyclop // Large function handling complex retry event delivery logic with many conditional branches
func ProcessRetryEventDelivery(deps EventDeliveryProcessorDeps) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Start a new trace span for retry event delivery
		traceStartTime := time.Now()
		attributes := map[string]interface{}{
			"event.type": "event.retry.delivery",
		}

		var data EventDelivery

		err := msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			innerErr := json.Unmarshal(t.Payload(), &data)
			if innerErr != nil {
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
				return &EndpointError{Err: innerErr, delay: defaultEventDelay}
			}
		}

		attributes["event_delivery.id"] = data.EventDeliveryID
		attributes["project.id"] = data.ProjectID

		cfg, err := config.Get()
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		eventDelivery, err := deps.EventDeliveryRepo.FindEventDeliveryByID(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		project, err := deps.ProjectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		endpoint, err := deps.EndpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			if errors.Is(err, datastore.ErrEndpointNotFound) {
				eventDelivery.Description = datastore.ErrEndpointNotFound.Error()
				innerErr := deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
				if innerErr != nil {
					deps.Logger.ErrorContext(ctx, "failed to update event delivery status to discarded", "error", innerErr)
				}

				deps.TracerBackend.Capture(ctx, "event.retry.delivery.discarded", attributes, traceStartTime, time.Now())
				return nil
			}

			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		attributes["endpoint.url"] = endpoint.Url
		attributes["endpoint.id"] = endpoint.UID
		attributes["event.id"] = eventDelivery.EventID

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.success", attributes, traceStartTime, time.Now())
			return nil
		}

		err = deps.RateLimiter.AllowWithDuration(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			deps.Logger.DebugContext(ctx, fmt.Sprintf("too many events to %s, limit of %v reqs/%v has been reached", endpoint.Url, endpoint.RateLimit, time.Duration(endpoint.RateLimitDuration)*time.Second), "event_delivery_id", data.EventDeliveryID, "error", err)

			deps.TracerBackend.Capture(ctx, "event.retry.delivery.rate_limited", attributes, traceStartTime, time.Now())
			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		if deps.FeatureFlag.CanAccessFeature(fflag.CircuitBreaker) && deps.Licenser.CircuitBreaking() {
			breakerErr := deps.CircuitBreakerManager.CanExecute(ctx, endpoint.UID)
			if breakerErr != nil {
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.circuit_breaker", attributes, traceStartTime, time.Now())
				return &CircuitBreakerError{Err: breakerErr}
			}

			// check the circuit breaker state so we can disable the endpoint
			cb, breakerErr := deps.CircuitBreakerManager.GetCircuitBreaker(ctx, endpoint.UID)
			if breakerErr != nil {
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.circuit_breaker", attributes, traceStartTime, time.Now())
				return &CircuitBreakerError{Err: breakerErr}
			}

			if cb != nil {
				if cb.ConsecutiveFailures > deps.CircuitBreakerManager.GetConfig().ConsecutiveFailureThreshold {
					endpointStatus := datastore.InactiveEndpointStatus

					breakerErr = deps.EndpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
					if breakerErr != nil {
						deps.Logger.ErrorContext(ctx, "failed to deactivate endpoint after failed retry", "error", breakerErr)
					}
				}
			}
		}

		err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		var attempt datastore.DeliveryAttempt
		done := true

		if eventDelivery.Status == datastore.SuccessEventStatus {
			deps.Logger.DebugContext(ctx, fmt.Sprintf("endpoint %s already merged with message %s\n", endpoint.Url, eventDelivery.UID))
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.success", attributes, traceStartTime, time.Now())
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
				return &EndpointError{Err: err, delay: defaultEventDelay}
			}

			deps.Logger.DebugContext(ctx, fmt.Sprintf("endpoint %s is inactive, failing to send.", endpoint.Url))
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.discarded", attributes, traceStartTime, time.Now())
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: err, delay: defaultEventDelay}
		}

		targetURL := endpoint.Url
		if !util.IsStringEmpty(eventDelivery.URLQueryParams) {
			targetURL, err = url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
			if err != nil {
				deps.Logger.ErrorContext(ctx, "failed to concat url query params", "error", err)
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
				return &EndpointError{Err: err, delay: defaultEventDelay}
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

		// Refresh OAuth2 token if endpoint uses OAuth2 authentication
		if endpoint.Authentication != nil && endpoint.Authentication.Type == datastore.OAuth2Authentication {
			// Check feature flag for OAuth2 using project's organisation ID
			oauth2Enabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, project.OrganisationID)
			if !oauth2Enabled {
				deps.Logger.WarnContext(ctx, "Endpoint has OAuth2 configured but feature flag is disabled, continuing without OAuth2 authentication")
				// Continue without OAuth2 authentication if feature flag is disabled
			} else if deps.OAuth2TokenService == nil {
				deps.Logger.ErrorContext(ctx, "OAuth2 token service is nil during retry")
			} else {
				authHeader, err := deps.OAuth2TokenService.GetAuthorizationHeader(ctx, endpoint)
				if err != nil {
					deps.Logger.ErrorContext(ctx, "failed to get OAuth2 authorization header for retry", "error", err)
				} else {
					if eventDelivery.Headers == nil {
						eventDelivery.Headers = httpheader.HTTPHeader{}
					}
					eventDelivery.Headers["Authorization"] = []string{authHeader}
					deps.Logger.InfoContext(ctx, "OAuth2 authorization header refreshed for retry", "endpoint.id", endpoint.UID)
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
				innerErr := deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
				if innerErr != nil {
					deps.Logger.ErrorContext(ctx, "failed to update event delivery status to failed", "error", innerErr)
				}
				deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
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
					innerErr := deps.EventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.FailureEventStatus)
					if innerErr != nil {
						deps.Logger.ErrorContext(ctx, "failed to update event delivery status to failed", "error", innerErr)
					}
					deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
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
		logAttrs := []any{"status", status, "uri", targetURL, "method", convoy.HttpPost, "duration", duration, "eventDeliveryID", eventDelivery.UID}

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			deps.Logger.DebugContext(ctx, fmt.Sprintf("%s sent", eventDelivery.UID), logAttrs...)
			attemptStatus = true

			eventDelivery.Status = datastore.SuccessEventStatus
			eventDelivery.Description = ""
		} else {
			deps.Logger.ErrorContext(ctx, eventDelivery.UID, logAttrs...)
			done = false

			// For at-most-once delivery, only retry on network failures
			if eventDelivery.DeliveryMode == datastore.AtMostOnceDeliveryMode {
				if retryableForAtMostOnceDeliveryMode(resp.StatusCode) {
					// Network error - retry
					eventDelivery.Status = datastore.RetryEventStatus
					nextTime := time.Now().Add(delayDuration)
					eventDelivery.Metadata.NextSendTime = nextTime
					attempts := eventDelivery.Metadata.NumTrials + 1

					deps.Logger.ErrorContext(ctx, fmt.Sprintf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", eventDelivery.UID,
						nextTime.Format(time.ANSIC), eventDelivery.Metadata.Strategy, eventDelivery.Metadata.IntervalSeconds, attempts, eventDelivery.Metadata.RetryLimit))
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

				deps.Logger.ErrorContext(ctx, fmt.Sprintf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", eventDelivery.UID,
					nextTime.Format(time.ANSIC), eventDelivery.Metadata.Strategy, eventDelivery.Metadata.IntervalSeconds, attempts, eventDelivery.Metadata.RetryLimit))
			}
		}

		// Update attributes with response info
		if resp != nil {
			attributes["response.status"] = resp.Status
			attributes["response.ip"] = resp.IP
			attributes["response.status_code"] = resp.StatusCode
			attributes["response.size_bytes"] = len(resp.Body)
		}

		// Request failed but statusCode is 200 <= x <= 299
		if err != nil {
			deps.Logger.ErrorContext(ctx, fmt.Sprintf("%s failed. Reason: %s", eventDelivery.UID, err))
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
		} else {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.success", attributes, traceStartTime, time.Now())
		}

		attempt = parseAttemptFromResponse(eventDelivery, endpoint, resp, attemptStatus)

		eventDelivery.Metadata.NumTrials++

		if eventDelivery.Metadata.NumTrials >= eventDelivery.Metadata.RetryLimit {
			if done {
				if eventDelivery.Status != datastore.SuccessEventStatus {
					deps.Logger.ErrorContext(ctx, "an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					eventDelivery.Status = datastore.FailureEventStatus
				}
			} else {
				deps.Logger.ErrorContext(ctx, fmt.Sprintf("%s retry limit exceeded ", eventDelivery.UID))
				eventDelivery.Description = "Retry limit exceeded"
				eventDelivery.Status = datastore.FailureEventStatus
			}

			if project.Config.DisableEndpoint && !deps.Licenser.CircuitBreaking() {
				endpointStatus := datastore.InactiveEndpointStatus

				err := deps.EndpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					deps.Logger.ErrorContext(ctx, "failed to deactivate endpoint after failed retry", "error", err)
				}

				if deps.Licenser.AdvancedEndpointMgmt() {
					// send endpoint deactivation notification
					err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, deps.Queue, true, resp.Error, string(resp.Body), resp.StatusCode, deps.Logger)
					if err != nil {
						deps.Logger.ErrorContext(ctx, "failed to send notification", "error", err)
					}
				}
			}
		}

		err = deps.AttemptsRepo.CreateDeliveryAttempt(ctx, &attempt)
		if err != nil {
			deps.Logger.ErrorContext(ctx, fmt.Sprintf("failed to create delivery attempt for event delivery with id: %s and delivery attempt: %s", eventDelivery.UID, attempt.ResponseData), "error", err)
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error())}
		}

		err = deps.EventDeliveryRepo.UpdateEventDeliveryMetadata(ctx, project.UID, eventDelivery)
		if err != nil {
			deps.Logger.ErrorContext(ctx, "failed to update message", "error", err, "event_delivery_uid", eventDelivery.UID)
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: fmt.Errorf("%s, err: %s", ErrDeliveryAttemptFailed, err.Error()), delay: defaultEventDelay}
		}

		if !done && eventDelivery.Metadata.NumTrials < eventDelivery.Metadata.RetryLimit {
			deps.TracerBackend.Capture(ctx, "event.retry.delivery.error", attributes, traceStartTime, time.Now())
			return &EndpointError{Err: fmt.Errorf("%s: delivery not completed, retrying", ErrDeliveryAttemptFailed), delay: delayDuration}
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
	responseHeader := datastore.ConvertDefaultHeaderToCustomHeader(&resp.ResponseHeader)
	requestHeader := datastore.ConvertDefaultHeaderToCustomHeader(&resp.RequestHeader)

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
		ResponseData:     resp.Body,
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
