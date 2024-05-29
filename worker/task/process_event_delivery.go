package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/database"
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
	ErrDeliveryAttemptFailed               = errors.New("error sending event")
	ErrRateLimit                           = errors.New("rate limit error")
	defaultDelay             time.Duration = 10
)

type SignatureValues struct {
	HMAC      string
	Timestamp string
}
type EventDelivery struct {
	EventDeliveryID string
	ProjectID       string
}

func ProcessEventDelivery(db database.Database, endpointRepo datastore.EndpointRepository, eventDeliveryRepo datastore.EventDeliveryRepository,
	projectRepo datastore.ProjectRepository, notificationQueue queue.Queuer, rateLimiter limiter.RateLimiter,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data EventDelivery

		err := msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &data)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		cfg, err := config.Get()
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		eventDelivery, err := eventDeliveryRepo.FindEventDeliveryByID(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*eventDelivery.Metadata).NextDuration(eventDelivery.Metadata.NumTrials)

		endpoint, err := endpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, eventDelivery.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		project, err := projectRepo.FetchProjectByID(ctx, eventDelivery.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		switch eventDelivery.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			return nil
		}

		err = rateLimiter.Allow(ctx, endpoint.UID, endpoint.RateLimit, int(endpoint.RateLimitDuration))
		if err != nil {
			log.FromContext(ctx).WithFields(map[string]interface{}{"event_delivery id": data.EventDeliveryID}).
				WithError(err).
				Debugf("too many events to %s, limit of %v reqs/%v has been reached", endpoint.Url, endpoint.RateLimit, time.Duration(endpoint.RateLimitDuration)*time.Second)

			return &RateLimitError{Err: ErrRateLimit, delay: time.Duration(endpoint.RateLimitDuration) * time.Second}
		}

		err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.ProcessingEventStatus)
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		var attempt datastore.DeliveryAttempt

		var httpDuration time.Duration
		if endpoint.HttpTimeout == 0 {
			httpDuration = convoy.HTTP_TIMEOUT_IN_DURATION
		} else {
			httpDuration = time.Duration(endpoint.HttpTimeout) * time.Second
		}

		done := true
		dispatch, err := net.NewDispatcher(httpDuration, cfg.Server.HTTP.HttpProxy, project.Config.SSL.EnforceSecureEndpoints)
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		if eventDelivery.Status == datastore.SuccessEventStatus {
			log.Debugf("endpoint %s already merged with message %s\n", endpoint.Url, eventDelivery.UID)
			return nil
		}

		if endpoint.Status == datastore.InactiveEndpointStatus {
			err = eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, project.UID, *eventDelivery, datastore.DiscardedEventStatus)
			if err != nil {
				return &EndpointError{Err: err, delay: delayDuration}
			}

			log.Debugf("endpoint %s is inactive, failing to send.", endpoint.Url)
			return nil
		}

		sig := newSignature(endpoint, project, json.RawMessage(eventDelivery.Metadata.Raw))
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		targetURL := endpoint.Url
		if !util.IsStringEmpty(eventDelivery.URLQueryParams) {
			targetURL, err = url.ConcatQueryParams(endpoint.Url, eventDelivery.URLQueryParams)
			if err != nil {
				log.WithError(err).Error("failed to concat url query params")
				return &EndpointError{Err: err, delay: delayDuration}
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

		resp, err := dispatch.SendRequest(ctx, targetURL, string(convoy.HttpPost), sig.Payload, project.Config.Signature.Header.String(), header, int64(cfg.MaxResponseSize), eventDelivery.Headers, eventDelivery.IdempotencyKey)

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
			eventDelivery.Latency = time.Since(eventDelivery.CreatedAt).String()
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
			err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, notificationQueue, false, resp.Error, string(resp.Body), resp.StatusCode)
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

			if endpoint.Status != datastore.PendingEndpointStatus && project.Config.DisableEndpoint {
				endpointStatus := datastore.InactiveEndpointStatus

				err := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, endpointStatus)
				if err != nil {
					log.WithError(err).Error("failed to deactivate endpoint after failed retry")
				}

				// send endpoint deactivation notification
				err = notifications.SendEndpointNotification(ctx, endpoint, project, endpointStatus, notificationQueue, true, resp.Error, string(resp.Body), resp.StatusCode)
				if err != nil {
					log.WithError(err).Error("failed to send notification")
				}
			}
		}

		err = eventDeliveryRepo.UpdateEventDeliveryWithAttempt(ctx, project.UID, *eventDelivery, attempt)
		if err != nil {
			log.WithError(err).Error("failed to update message ", eventDelivery.UID)
			return &EndpointError{Err: ErrDeliveryAttemptFailed, delay: delayDuration}
		}

		if !done && eventDelivery.Metadata.NumTrials < eventDelivery.Metadata.RetryLimit {
			return &EndpointError{Err: ErrDeliveryAttemptFailed, delay: delayDuration}
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
		UID:        ulid.Make().String(),
		URL:        resp.URL.String(),
		Method:     resp.Method,
		MsgID:      m.UID,
		EndpointID: e.UID,
		APIVersion: convoy.GetVersion(),

		IPAddress:        resp.IP,
		ResponseHeader:   *responseHeader,
		RequestHeader:    *requestHeader,
		HttpResponseCode: resp.Status,
		ResponseData:     string(resp.Body),
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
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

func (ec *EventDeliveryConfig) retryConfig() (*RetryConfig, error) {
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

func (ec *EventDeliveryConfig) rateLimitConfig() *RateLimitConfig {
	rlc := &RateLimitConfig{}

	rlc.Rate = ec.endpoint.RateLimit
	rlc.BucketSize = int(ec.endpoint.RateLimitDuration)

	return rlc
}
