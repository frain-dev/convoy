package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/disq"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrDeliveryAttemptFailed = errors.New("error sending event")
var defaultDelay time.Duration = 30

type SignatureValues struct {
	HMAC      string
	Timestamp string
}

func ProcessEventDelivery(
	appRepo datastore.ApplicationRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	groupRepo datastore.GroupRepository,
	rateLimiter limiter.RateLimiter,
	subRepo datastore.SubscriptionRepository,
) func(*queue.Job) error {
	return func(job *queue.Job) error {
		Id := job.ID

		// Load message from DB and switch state to prevent concurrent processing.
		ed, err := eventDeliveryRepo.FindEventDeliveryByID(context.Background(), Id)
		if err != nil {
			log.WithError(err).Errorf("Failed to load event - %s", Id)
			return &disq.Error{Err: err, Delay: defaultDelay}
		}

		endpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), ed.AppID, ed.EndpointID)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		app, err := appRepo.FindApplicationByID(context.Background(), ed.AppID)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		subscription, err := subRepo.FindSubscriptionByID(context.Background(), ed.GroupID, ed.SubscriptionID)
		if err != nil {
			println(err)
			fmt.Printf("\nErr: %+v\nEd: %+v\n\n", err, ed)
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		var delayDuration time.Duration = retrystrategies.NewRetryStrategyFromMetadata(*ed.Metadata).NextDuration(ed.Metadata.NumTrials)

		switch ed.Status {
		case datastore.ProcessingEventStatus,
			datastore.SuccessEventStatus:
			return nil
		}

		var rateLimitDuration time.Duration
		if util.IsStringEmpty(endpoint.RateLimitDuration) {
			rateLimitDuration, err = time.ParseDuration(convoy.RATE_LIMIT_DURATION)
			if err != nil {
				log.WithError(err).Errorf("failed to parse endpoint rate limit")
				return nil
			}
		} else {
			rateLimitDuration, err = time.ParseDuration(endpoint.RateLimitDuration)
			if err != nil {
				log.WithError(err).Errorf("failed to parse endpoint rate limit")
				return nil
			}
		}

		var rateLimit int
		if endpoint.RateLimit == 0 {
			rateLimit = convoy.RATE_LIMIT
		} else {
			rateLimit = endpoint.RateLimit
		}

		res, err := rateLimiter.ShouldAllow(context.Background(), endpoint.TargetURL, rateLimit, int(rateLimitDuration))
		if err != nil {
			return nil
		}

		if res.Remaining <= 0 {
			err := fmt.Errorf("too many events to %s, limit of %v would be reached", endpoint.TargetURL, res.Limit)
			log.WithError(errors.New("rate limit error")).Error(err.Error())

			var delayDuration time.Duration = retrystrategies.NewRetryStrategyFromMetadata(*ed.Metadata).NextDuration(ed.Metadata.NumTrials)
			return &disq.Error{Err: err, Delay: delayDuration, RateLimit: true}
		}

		_, err = rateLimiter.Allow(context.Background(), endpoint.TargetURL, rateLimit, int(rateLimitDuration))
		if err != nil {
			return nil
		}

		err = eventDeliveryRepo.UpdateStatusOfEventDelivery(context.Background(), *ed, datastore.ProcessingEventStatus)
		if err != nil {
			log.WithError(err).Error("failed to update status of messages - ")
			return &disq.Error{Err: err, Delay: delayDuration}
		}

		var attempt datastore.DeliveryAttempt
		var secret = endpoint.Secret

		cfg, err := config.Get()
		if err != nil {
			return &disq.Error{Err: err, Delay: delayDuration}
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

		dispatch := net.NewDispatcher(httpDuration)

		var done = true

		e := endpoint
		if ed.Status == datastore.SuccessEventStatus {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, ed.UID)
			return nil
		}

		if subscription.Status == datastore.InactiveEndpointStatus {
			log.Debugf("subscription %s is inactive, failing to send.", e.TargetURL)
			return nil
		}

		buff := bytes.NewBuffer([]byte{})
		encoder := json.NewEncoder(buff)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(ed.Metadata.Data); err != nil {
			log.WithError(err).Error("Failed to encode data")
			return &disq.Error{Err: err, Delay: delayDuration}
		}

		bStr := strings.TrimSuffix(buff.String(), "\n")

		g, err := groupRepo.FetchGroupByID(context.Background(), app.GroupID)
		if err != nil {
			log.WithError(err).Error("could not find error")
			return &disq.Error{Err: err, Delay: delayDuration}
		}
		var signedPayload strings.Builder
		var timestamp string
		if g.Config.ReplayAttacks {
			timestamp = fmt.Sprint(time.Now().Unix())
			signedPayload.WriteString(timestamp)
			signedPayload.WriteString(",")
		}
		signedPayload.WriteString(bStr)

		hmac, err := util.ComputeJSONHmac(g.Config.Signature.Hash, signedPayload.String(), secret, false)
		if err != nil {
			log.Errorf("error occurred while generating hmac - %+v\n", err)
			return &disq.Error{Err: err, Delay: delayDuration}
		}

		attemptStatus := false
		start := time.Now()

		resp, err := dispatch.SendRequest(e.TargetURL, string(convoy.HttpPost), []byte(bStr), g, hmac, timestamp, int64(cfg.MaxResponseSize))
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

		if done && subscription.Status == datastore.PendingEndpointStatus && g.Config.DisableEndpoint {
			endpoints := []string{endpoint.UID}
			endpointStatus := datastore.ActiveEndpointStatus

			err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), app.UID, endpoints, endpointStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}

			err = sendNotification(context.Background(), appRepo, ed, g, &cfg.SMTP, endpointStatus, false)
			if err != nil {
				log.WithError(err).Error("failed to send notification")
			}
		}

		if !done && subscription.Status == datastore.PendingEndpointStatus {
			endpoints := []string{endpoint.UID}
			endpointStatus := datastore.InactiveEndpointStatus

			err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), app.UID, endpoints, endpointStatus)
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

			subscriptionStatus := subscription.Status
			if g.Config.DisableEndpoint && subscription.Status != datastore.PendingEndpointStatus {
				endpoints := []string{endpoint.UID}
				subscriptionStatus = datastore.InactiveEndpointStatus

				err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), app.UID, endpoints, subscriptionStatus)
				if err != nil {
					log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
				}
			}

			err = sendNotification(context.Background(), appRepo, ed, g, &cfg.SMTP, subscriptionStatus, true)
			if err != nil {
				log.WithError(err).Error("failed to send notification")
			}
		}

		err = eventDeliveryRepo.UpdateEventDeliveryWithAttempt(context.Background(), *ed, attempt)
		if err != nil {
			log.WithError(err).Error("failed to update message ", ed.UID)
		}

		if !done && ed.Metadata.NumTrials < ed.Metadata.RetryLimit {
			return &disq.Error{Err: ErrDeliveryAttemptFailed, Delay: delayDuration}
		}

		return nil
	}
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
