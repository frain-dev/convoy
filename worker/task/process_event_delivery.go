package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/signature"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrDeliveryAttemptFailed               = errors.New("error sending event")
	ErrRateLimit                           = errors.New("rate limit error")
	defaultDelay             time.Duration = 30
)

type SignatureValues struct {
	HMAC      string
	Timestamp string
}

func ProcessEventDelivery(appRepo datastore.ApplicationRepository, eventDeliveryRepo datastore.EventDeliveryRepository, groupRepo datastore.GroupRepository, rateLimiter limiter.RateLimiter, subRepo datastore.SubscriptionRepository, notificationQueue queue.Queuer) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		Id := string(t.Payload())

		// Load message from DB and switch state to prevent concurrent processing.
		ed, err := eventDeliveryRepo.FindEventDeliveryByID(context.Background(), Id)
		if err != nil {
			log.WithError(err).Errorf("Failed to load event - %s", Id)
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		endpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), ed.AppID, ed.EndpointID)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		app, err := appRepo.FindApplicationByID(context.Background(), ed.AppID)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		subscription, err := subRepo.FindSubscriptionByID(context.Background(), ed.GroupID, ed.SubscriptionID)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*ed.Metadata).NextDuration(ed.Metadata.NumTrials)

		g, err := groupRepo.FetchGroupByID(context.Background(), app.GroupID)
		if err != nil {
			log.WithError(err).Error("could not find error")
			return &EndpointError{Err: err, delay: delayDuration}
		}

		switch ed.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			return nil
		}

		ec := &EventDeliveryConfig{subscription: subscription, group: g}
		rlc := ec.rateLimitConfig()

		res, err := rateLimiter.ShouldAllow(context.Background(), endpoint.TargetURL, rlc.Count, int(rlc.Duration))
		if err != nil {
			return nil
		}

		if res.Remaining <= 0 {
			err := fmt.Errorf("too many events to %s, limit of %v would be reached", endpoint.TargetURL, res.Limit)
			log.WithError(ErrRateLimit).Error(err.Error())

			var delayDuration time.Duration = retrystrategies.NewRetryStrategyFromMetadata(*ed.Metadata).NextDuration(ed.Metadata.NumTrials)
			return &RateLimitError{Err: ErrRateLimit, delay: delayDuration}
		}

		_, err = rateLimiter.Allow(context.Background(), endpoint.TargetURL, rlc.Count, int(rlc.Duration))
		if err != nil {
			return nil
		}

		err = eventDeliveryRepo.UpdateStatusOfEventDelivery(context.Background(), *ed, datastore.ProcessingEventStatus)
		if err != nil {
			log.WithError(err).Error("failed to update status of messages - ")
			return &EndpointError{Err: err, delay: delayDuration}
		}

		var attempt datastore.DeliveryAttempt

		cfg, err := config.Get()
		if err != nil {
			return &EndpointError{Err: err, delay: delayDuration}
		}

		var httpDuration time.Duration
		if util.IsStringEmpty(endpoint.HttpTimeout) {
			httpDuration, err = time.ParseDuration(convoy.HTTP_TIMEOUT)
			if err != nil {
				log.WithError(err).Errorf("failed to parse endpoint duration")
				return nil
			}
		} else {
			httpDuration, err = time.ParseDuration(endpoint.HttpTimeout)
			if err != nil {
				log.WithError(err).Errorf("failed to parse endpoint duration")
				return nil
			}
		}

		done := true
		dispatch, err := net.NewDispatcher(httpDuration, cfg.Server.HTTP.HttpProxy)
		if err != nil {
			log.Errorf("error occurred while creating the http client - %+v\n", err)
			return &EndpointError{Err: err, delay: delayDuration}
		}

		e := endpoint
		if ed.Status == datastore.SuccessEventStatus {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, ed.UID)
			return nil
		}

		if subscription.Status == datastore.InactiveSubscriptionStatus {
			log.Debugf("subscription %s is inactive, failing to send.", e.TargetURL)
			return nil
		}

		sig := newSignature(endpoint, g, ed.Metadata.Data)
		header, err := sig.ComputeHeaderValue()
		if err != nil {
			log.Errorf("error occurred while generating hmac - %+v\n", err)
			return &EndpointError{Err: err, delay: delayDuration}
		}

		attemptStatus := false
		start := time.Now()

		resp, err := dispatch.SendRequest(e.TargetURL, string(convoy.HttpPost), sig.Payload, g, header, int64(cfg.MaxResponseSize), ed.Headers)
		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(start)
		// log request details
		requestLogger := log.WithFields(log.Fields{
			"status":   status,
			"uri":      e.TargetURL,
			"method":   convoy.HttpPost,
			"duration": duration,
		})

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			requestLogger.Infof("%s", ed.UID)
			log.Infof("%s sent", ed.UID)
			attemptStatus = true
			// e.Sent = true

			ed.Status = datastore.SuccessEventStatus
			ed.Description = ""
		} else {
			requestLogger.Errorf("%s", ed.UID)
			done = false
			// e.Sent = false

			ed.Status = datastore.RetryEventStatus

			nextTime := time.Now().Add(delayDuration)
			ed.Metadata.NextSendTime = primitive.NewDateTimeFromTime(nextTime)
			attempts := ed.Metadata.NumTrials + 1

			log.Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", ed.UID, nextTime.Format(time.ANSIC), ed.Metadata.Strategy, ed.Metadata.IntervalSeconds, attempts, ed.Metadata.RetryLimit)
		}

		// Request failed but statusCode is 200 <= x <= 299
		if err != nil {
			log.Errorf("%s failed. Reason: %s", ed.UID, err)
		}

		if done && subscription.Status == datastore.PendingSubscriptionStatus && ec.disableEndpoint() {
			subscriptionStatus := datastore.ActiveSubscriptionStatus
			err := subRepo.UpdateSubscriptionStatus(context.Background(), g.UID, subscription.UID, subscriptionStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}

			// send endpoint reactivation notification
			err = notifications.SendEndpointNotification(context.Background(), app, endpoint, g, subscriptionStatus, notificationQueue, false)
			if err != nil {
				log.WithError(err).Error("failed to send notification")
			}
		}

		if !done && subscription.Status == datastore.PendingSubscriptionStatus {
			subscriptionStatus := datastore.InactiveSubscriptionStatus
			err := subRepo.UpdateSubscriptionStatus(context.Background(), g.UID, subscription.UID, subscriptionStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}
		}

		attempt = parseAttemptFromResponse(ed, endpoint, resp, attemptStatus)

		ed.Metadata.NumTrials++

		if ed.Metadata.NumTrials >= ed.Metadata.RetryLimit {
			if done {
				if ed.Status != datastore.SuccessEventStatus {
					log.Errorln("an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					ed.Status = datastore.FailureEventStatus
				}
			} else {
				log.Errorf("%s retry limit exceeded ", ed.UID)
				ed.Description = "Retry limit exceeded"
				ed.Status = datastore.FailureEventStatus
			}

			if ec.disableEndpoint() && subscription.Status != datastore.PendingSubscriptionStatus {
				subscriptionStatus := datastore.InactiveSubscriptionStatus

				err := subRepo.UpdateSubscriptionStatus(context.Background(), g.UID, subscription.UID, subscriptionStatus)
				if err != nil {
					log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
				}

				// send endpoint deactivation notification
				err = notifications.SendEndpointNotification(context.Background(), app, endpoint, g, subscriptionStatus, notificationQueue, true)
				if err != nil {
					log.WithError(err).Error("failed to send notification")
				}
			}
		}

		err = eventDeliveryRepo.UpdateEventDeliveryWithAttempt(context.Background(), *ed, attempt)
		if err != nil {
			log.WithError(err).Error("failed to update message ", ed.UID)
		}

		if !done && ed.Metadata.NumTrials < ed.Metadata.RetryLimit {
			return &EndpointError{Err: ErrDeliveryAttemptFailed, delay: delayDuration}
		}

		return nil
	}
}

func newSignature(endpoint *datastore.Endpoint, g *datastore.Group, data json.RawMessage) *signature.Signature {
	s := &signature.Signature{Advanced: endpoint.AdvancedSignatures, Payload: data}

	for _, version := range g.Config.Signature.Versions {
		scheme := signature.Scheme{
			Hash:     version.Hash,
			Encoding: version.Encoding.String(),
		}

		for _, sc := range endpoint.Secrets {
			if sc.DeletedAt == 0 {
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
		ID:         primitive.NewObjectID(),
		UID:        uuid.New().String(),
		URL:        resp.URL.String(),
		Method:     resp.Method,
		MsgID:      m.UID,
		EndpointID: e.UID,
		APIVersion: "2021-08-27",

		IPAddress:        resp.IP,
		ResponseHeader:   *responseHeader,
		RequestHeader:    *requestHeader,
		HttpResponseCode: resp.Status,
		ResponseData:     string(resp.Body),
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
}

type EventDeliveryConfig struct {
	group        *datastore.Group
	subscription *datastore.Subscription
}

type RetryConfig struct {
	Type       datastore.StrategyProvider
	Duration   uint64
	RetryCount uint64
}

type RateLimitConfig struct {
	Count    int
	Duration uint64
}

func (ec *EventDeliveryConfig) disableEndpoint() bool {
	if ec.subscription.DisableEndpoint != nil {
		return *ec.subscription.DisableEndpoint
	}

	return ec.group.Config.DisableEndpoint
}

func (ec *EventDeliveryConfig) retryConfig() (*RetryConfig, error) {
	rc := &RetryConfig{}

	if ec.subscription.RetryConfig != nil {
		rc.Duration = ec.subscription.RetryConfig.Duration
		rc.RetryCount = ec.subscription.RetryConfig.RetryCount
		rc.Type = ec.subscription.RetryConfig.Type
	} else {
		rc.Duration = ec.group.Config.Strategy.Duration
		rc.RetryCount = ec.group.Config.Strategy.RetryCount
		rc.Type = ec.group.Config.Strategy.Type
	}

	return rc, nil
}

func (ec *EventDeliveryConfig) rateLimitConfig() *RateLimitConfig {
	rlc := &RateLimitConfig{}

	if ec.subscription.RateLimitConfig != nil {
		rlc.Count = ec.subscription.RateLimitConfig.Count
		rlc.Duration = ec.subscription.RateLimitConfig.Duration
	} else {
		rlc.Count = ec.group.Config.RateLimit.Count
		rlc.Duration = ec.group.Config.RateLimit.Duration
	}

	return rlc
}
